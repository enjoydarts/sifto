package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
)

const cacheMetricTTL = 8 * 24 * time.Hour

type DashboardHandler struct {
	sourceRepo   *repository.SourceRepo
	itemRepo     *repository.ItemRepo
	digestRepo   *repository.DigestRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cache        service.JSONCache
}

func NewDashboardHandler(sourceRepo *repository.SourceRepo, itemRepo *repository.ItemRepo, digestRepo *repository.DigestRepo, llmUsageRepo *repository.LLMUsageLogRepo, cache service.JSONCache) *DashboardHandler {
	return &DashboardHandler{
		sourceRepo:   sourceRepo,
		itemRepo:     itemRepo,
		digestRepo:   digestRepo,
		llmUsageRepo: llmUsageRepo,
		cache:        cache,
	}
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	llmDays := parseIntOrDefault(r.URL.Query().Get("llm_days"), 7)
	if llmDays < 1 || llmDays > 365 {
		http.Error(w, "invalid llm_days", http.StatusBadRequest)
		return
	}
	topicLimit := parseIntOrDefault(r.URL.Query().Get("topic_limit"), 8)
	if topicLimit < 1 || topicLimit > 50 {
		http.Error(w, "invalid topic_limit", http.StatusBadRequest)
		return
	}
	digestLimit := parseIntOrDefault(r.URL.Query().Get("digest_limit"), 5)
	if digestLimit < 1 || digestLimit > 20 {
		http.Error(w, "invalid digest_limit", http.StatusBadRequest)
		return
	}
	cacheKey := fmt.Sprintf("dashboard:%s:llm%d:topic%d:digest%d", userID, llmDays, topicLimit, digestLimit)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			dashboardCacheCounter.hits.Add(1)
			_ = h.cache.IncrMetric(r.Context(), "cache", "dashboard.hit", 1, time.Now(), cacheMetricTTL)
			log.Printf("dashboard cache hit user_id=%s key=%s", userID, cacheKey)
			writeJSON(w, cached)
			return
		} else if err != nil {
			dashboardCacheCounter.errors.Add(1)
			_ = h.cache.IncrMetric(r.Context(), "cache", "dashboard.error", 1, time.Now(), cacheMetricTTL)
			log.Printf("dashboard cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		dashboardCacheCounter.misses.Add(1)
		_ = h.cache.IncrMetric(r.Context(), "cache", "dashboard.miss", 1, time.Now(), cacheMetricTTL)
		log.Printf("dashboard cache miss user_id=%s key=%s", userID, cacheKey)
	} else if cacheBust {
		dashboardCacheCounter.bypass.Add(1)
		if h.cache != nil {
			_ = h.cache.IncrMetric(r.Context(), "cache", "dashboard.bypass", 1, time.Now(), cacheMetricTTL)
		}
		log.Printf("dashboard cache bypass user_id=%s key=%s", userID, cacheKey)
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		firstErr    error
		sourceCnt   int
		itemStats   any
		digests     any
		llmSummary  any
		topics      any
		failedItems any
	)
	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	wg.Add(6)
	go func() {
		defer wg.Done()
		n, err := h.sourceRepo.CountByUser(r.Context(), userID)
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		sourceCnt = n
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		v, err := h.itemRepo.Stats(r.Context(), userID)
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		itemStats = v
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		v, err := h.digestRepo.ListLimit(r.Context(), userID, digestLimit)
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		digests = v
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		v, err := h.llmUsageRepo.DailySummaryByUser(r.Context(), userID, llmDays)
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		llmSummary = v
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		v, err := h.itemRepo.TopicTrends(r.Context(), userID, topicLimit)
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		topics = v
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		status := "failed"
		v, err := h.itemRepo.ListPage(r.Context(), userID, repository.ItemListParams{
			Status:   &status,
			Sort:     "newest",
			Page:     1,
			PageSize: 5,
		})
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		failedItems = v
		mu.Unlock()
	}()
	wg.Wait()
	if firstErr != nil {
		writeRepoError(w, firstErr)
		return
	}

	resp := map[string]any{
		"sources_count": sourceCnt,
		"item_stats":    itemStats,
		"digests":       digests,
		"llm_summary":   llmSummary,
		"topic_trends": map[string]any{
			"items":  topics,
			"limit":  topicLimit,
			"period": "24h_vs_prev24h",
		},
		"failed_items_preview": failedItems,
		"llm_days":             llmDays,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, 30*time.Second); err != nil {
			dashboardCacheCounter.errors.Add(1)
			_ = h.cache.IncrMetric(r.Context(), "cache", "dashboard.error", 1, time.Now(), cacheMetricTTL)
			log.Printf("dashboard cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}
