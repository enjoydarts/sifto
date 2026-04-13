package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

const cacheMetricTTL = 8 * 24 * time.Hour
const dashboardCacheTTL = 120 * time.Second
const dashboardPartCacheTTL = 5 * time.Minute

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
	cacheKey := cacheKeyDashboard(userID, llmDays, topicLimit, digestLimit)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			dashboardCacheCounter.hits.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "dashboard.hit")
			log.Printf("dashboard cache hit user_id=%s key=%s", userID, cacheKey)
			writeJSON(w, cached)
			return
		} else if err != nil {
			dashboardCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "dashboard.error")
			log.Printf("dashboard cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		dashboardCacheCounter.misses.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "dashboard.miss")
		log.Printf("dashboard cache miss user_id=%s key=%s", userID, cacheKey)
	} else if cacheBust {
		dashboardCacheCounter.bypass.Add(1)
		if h.cache != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "dashboard.bypass")
		}
		log.Printf("dashboard cache bypass user_id=%s key=%s", userID, cacheKey)
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		firstErr    error
		sourceCnt   any
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
	loadPart := func(part, key string, fetch func() (any, error), assign func(any)) {
		if h.cache != nil && !cacheBust {
			var cached any
			if ok, err := h.cache.GetJSON(r.Context(), key, &cached); err == nil && ok {
				incrCacheMetric(r.Context(), h.cache, userID, fmt.Sprintf("dashboard_part.%s.hit", part))
				mu.Lock()
				assign(cached)
				mu.Unlock()
				return
			} else if err != nil {
				incrCacheMetric(r.Context(), h.cache, userID, fmt.Sprintf("dashboard_part.%s.error", part))
				log.Printf("dashboard-part cache get failed user_id=%s part=%s key=%s err=%v", userID, part, key, err)
			}
			incrCacheMetric(r.Context(), h.cache, userID, fmt.Sprintf("dashboard_part.%s.miss", part))
		} else if cacheBust && h.cache != nil {
			incrCacheMetric(r.Context(), h.cache, userID, fmt.Sprintf("dashboard_part.%s.bypass", part))
		}
		v, err := fetch()
		if err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		assign(v)
		mu.Unlock()
		if h.cache != nil {
			if err := h.cache.SetJSON(r.Context(), key, v, dashboardPartCacheTTL); err != nil {
				incrCacheMetric(r.Context(), h.cache, userID, fmt.Sprintf("dashboard_part.%s.error", part))
				log.Printf("dashboard-part cache set failed user_id=%s part=%s key=%s err=%v", userID, part, key, err)
			}
		}
	}

	wg.Add(6)
	safeGo(func() {
		defer wg.Done()
		partKey := cacheKeyDashboardPart(userID, "sources", 0, 0)
		loadPart("sources", partKey, func() (any, error) {
			n, err := h.sourceRepo.CountByUser(r.Context(), userID)
			if err != nil {
				return nil, err
			}
			return n, nil
		}, func(v any) { sourceCnt = v })
	})
	safeGo(func() {
		defer wg.Done()
		partKey := cacheKeyDashboardPart(userID, "itemstats", 0, 0)
		loadPart("itemstats", partKey, func() (any, error) {
			return h.itemRepo.Stats(r.Context(), userID)
		}, func(v any) { itemStats = v })
	})
	safeGo(func() {
		defer wg.Done()
		partKey := cacheKeyDashboardPart(userID, "digests", digestLimit, 0)
		loadPart("digests", partKey, func() (any, error) {
			return h.digestRepo.ListLimit(r.Context(), userID, digestLimit)
		}, func(v any) { digests = v })
	})
	safeGo(func() {
		defer wg.Done()
		partKey := cacheKeyDashboardPart(userID, "llm", llmDays, 0)
		loadPart("llm", partKey, func() (any, error) {
			return h.llmUsageRepo.DailySummaryByUser(r.Context(), userID, llmDays)
		}, func(v any) { llmSummary = v })
	})
	safeGo(func() {
		defer wg.Done()
		partKey := cacheKeyDashboardPart(userID, "topics", topicLimit, 0)
		loadPart("topics", partKey, func() (any, error) {
			return h.itemRepo.TopicTrends(r.Context(), userID, topicLimit)
		}, func(v any) { topics = v })
	})
	safeGo(func() {
		defer wg.Done()
		partKey := cacheKeyDashboardPart(userID, "failedpreview", 0, 0)
		loadPart("failedpreview", partKey, func() (any, error) {
			status := "failed"
			return h.itemRepo.ListPage(r.Context(), userID, repository.ItemListParams{
				Status:   &status,
				Sort:     "newest",
				Page:     1,
				PageSize: 5,
			})
		}, func(v any) { failedItems = v })
	})
	wg.Wait()
	if firstErr != nil {
		writeRepoError(w, firstErr)
		return
	}

	resp := dashboardResponse{
		SourcesCount: sourceCnt,
		ItemStats:    itemStats,
		Digests:      digests,
		LLMSummary:   llmSummary,
		TopicTrends: dashboardTopicTrends{
			Items:  topics,
			Limit:  topicLimit,
			Period: "24h_vs_prev24h",
		},
		FailedItemsPreview: failedItems,
		LLMDays:            llmDays,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, dashboardCacheTTL); err != nil {
			dashboardCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "dashboard.error")
			log.Printf("dashboard cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}
