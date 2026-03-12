package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/go-chi/chi/v5"
)

type ItemHandler struct {
	repo       *repository.ItemRepo
	sourceRepo *repository.SourceRepo
	streakRepo *repository.ReadingStreakRepo
	publisher  *service.EventPublisher
	cache      service.JSONCache
	detail     *service.ItemDetailService
}

const itemsListCacheTTL = 30 * time.Second
const focusQueueCacheTTL = 60 * time.Second
const triageAllCacheTTL = 90 * time.Second
const relatedItemsCacheTTL = 5 * time.Minute

func NewItemHandler(
	repo *repository.ItemRepo,
	sourceRepo *repository.SourceRepo,
	streakRepo *repository.ReadingStreakRepo,
	publisher *service.EventPublisher,
	cache service.JSONCache,
) *ItemHandler {
	return &ItemHandler{
		repo:       repo,
		sourceRepo: sourceRepo,
		streakRepo: streakRepo,
		publisher:  publisher,
		cache:      cache,
		detail:     service.NewItemDetailService(repo),
	}
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	var status, sourceID, topic *string
	if v := q.Get("status"); v != "" {
		status = &v
	}
	if v := q.Get("source_id"); v != "" {
		sourceID = &v
	}
	if v := q.Get("topic"); v != "" {
		topic = &v
	}
	page := parseIntOrDefault(q.Get("page"), 1)
	pageSize := parseIntOrDefault(q.Get("page_size"), 20)
	if page < 1 || page > 100000 {
		http.Error(w, "invalid page", http.StatusBadRequest)
		return
	}
	if pageSize < 1 || pageSize > 200 {
		http.Error(w, "invalid page_size", http.StatusBadRequest)
		return
	}
	sort := q.Get("sort")
	if sort == "" {
		sort = "newest"
	}
	if sort != "newest" && sort != "score" {
		http.Error(w, "invalid sort", http.StatusBadRequest)
		return
	}
	unreadOnly := q.Get("unread_only") == "true"
	readOnly := q.Get("read_only") == "true"
	favoriteOnly := q.Get("favorite_only") == "true"
	laterOnly := q.Get("later_only") == "true"
	if unreadOnly && readOnly {
		http.Error(w, "unread_only and read_only cannot both be true", http.StatusBadRequest)
		return
	}
	cacheKey := cacheKeyItemsList(userID, q.Get("status"), q.Get("source_id"), q.Get("topic"), unreadOnly, readOnly, favoriteOnly, laterOnly, sort, page, pageSize)
	cacheBust := q.Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.ItemListResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			itemsListCacheCounter.hits.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.hit")
			writeJSON(w, &cached)
			return
		} else if err != nil {
			itemsListCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.error")
			log.Printf("items-list cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		itemsListCacheCounter.misses.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "items_list.miss")
	} else if cacheBust {
		itemsListCacheCounter.bypass.Add(1)
		if h.cache != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.bypass")
		}
	}

	resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
		Status:       status,
		SourceID:     sourceID,
		Topic:        topic,
		UnreadOnly:   unreadOnly,
		ReadOnly:     readOnly,
		FavoriteOnly: favoriteOnly,
		LaterOnly:    laterOnly,
		Sort:         sort,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.cache != nil && resp != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, itemsListCacheTTL); err != nil {
			itemsListCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.error")
			log.Printf("items-list cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) Stats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	resp, err := h.repo.Stats(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) UXMetrics(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 7)
	if days < 1 || days > 90 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	today := timeutil.StartOfDayJST(timeutil.NowJST())
	todayStr := today.Format("2006-01-02")
	fromStr := today.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	todayNew, err := h.repo.CountNewOnDateJST(r.Context(), userID, todayStr)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	todayRead, err := h.repo.CountReadOnDateJST(r.Context(), userID, todayStr)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	periodRead, activeDays, err := h.repo.ReadActivityInRangeJST(r.Context(), userID, fromStr, todayStr)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	var todayRate *float64
	if todayNew > 0 {
		v := float64(todayRead) / float64(todayNew)
		todayRate = &v
	}
	avgReads := float64(periodRead) / float64(days)

	streak := 0
	if h.streakRepo != nil {
		if _, streakDays, _, err := h.streakRepo.GetByUserAndDate(r.Context(), userID, todayStr); err == nil {
			streak = streakDays
		} else {
			yesterdayStr := today.AddDate(0, 0, -1).Format("2006-01-02")
			if _, streakDays, _, err := h.streakRepo.GetByUserAndDate(r.Context(), userID, yesterdayStr); err == nil {
				streak = streakDays
			}
		}
	}

	writeJSON(w, &model.ItemUXMetricsResponse{
		Days:                     days,
		TodayDate:                todayStr,
		TodayNewItems:            todayNew,
		TodayReadItems:           todayRead,
		TodayConsumptionRate:     todayRate,
		PeriodReadItems:          periodRead,
		PeriodActiveReadDays:     activeDays,
		PeriodAverageReadsPerDay: avgReads,
		CurrentStreakDays:        streak,
	})
}

func (h *ItemHandler) invalidateUserCaches(ctx context.Context, userID string) {
	if h.cache == nil || userID == "" {
		return
	}
	for _, prefix := range cacheUserInvalidatePrefixes(userID) {
		if _, err := h.cache.DeleteByPrefix(ctx, prefix, 5000); err != nil {
			log.Printf("cache invalidate failed user_id=%s prefix=%s err=%v", userID, prefix, err)
		}
	}
}

func (h *ItemHandler) TopicTrends(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 8)
	if limit < 1 || limit > 50 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.TopicTrends(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"items": rows,
		"limit": limit,
	})
}

func (h *ItemHandler) TopicPulse(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 7)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 12)
	if days < 1 || days > 30 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	if limit < 1 || limit > 50 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.TopicPulse(r.Context(), userID, days, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"days":  days,
		"limit": limit,
		"items": rows,
	})
}

func (h *ItemHandler) ReadingPlan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	window := q.Get("window")
	if window == "" {
		window = "24h"
	}
	size := parseIntOrDefault(q.Get("size"), 15)
	if size < 1 || size > 100 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	diversify := q.Get("diversify_topics") != "false"
	excludeRead := q.Get("exclude_read") != "false"
	params := repository.ReadingPlanParams{
		Window:          window,
		Size:            size,
		DiversifyTopics: diversify,
		ExcludeRead:     excludeRead,
		ExcludeLater:    q.Get("exclude_later") == "true",
	}
	cacheKey := cacheKeyReadingPlan(userID, params.Window, params.Size, params.DiversifyTopics, params.ExcludeRead, params.ExcludeLater)
	cacheBust := q.Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.ReadingPlanResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			readingPlanCacheCounter.hits.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.hit")
			log.Printf("reading-plan cache hit user_id=%s key=%s", userID, cacheKey)
			writeJSON(w, &cached)
			return
		} else if err != nil {
			readingPlanCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.error")
			log.Printf("reading-plan cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		readingPlanCacheCounter.misses.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.miss")
		log.Printf("reading-plan cache miss user_id=%s key=%s", userID, cacheKey)
	} else if cacheBust {
		readingPlanCacheCounter.bypass.Add(1)
		if h.cache != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.bypass")
		}
		log.Printf("reading-plan cache bypass user_id=%s key=%s", userID, cacheKey)
	}

	resp, err := h.repo.ReadingPlan(r.Context(), userID, params)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.cache != nil && resp != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, 120*time.Second); err != nil {
			readingPlanCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.error")
			log.Printf("reading-plan cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) FocusQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	window := q.Get("window")
	if window == "" {
		window = "24h"
	}
	size := parseIntOrDefault(q.Get("size"), 20)
	if size < 1 || size > 100 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	params := repository.ReadingPlanParams{
		Window:          window,
		Size:            size,
		DiversifyTopics: q.Get("diversify_topics") != "false",
		ExcludeRead:     false,
		ExcludeLater:    q.Get("exclude_later") != "false",
	}
	cacheKey := cacheKeyFocusQueue(userID, params.Window, params.Size, params.DiversifyTopics, params.ExcludeLater)
	cacheBust := q.Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.error")
			log.Printf("focus-queue cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.bypass")
	}

	resp, err := h.repo.ReadingPlan(r.Context(), userID, params)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if resp == nil {
		out := map[string]any{
			"items":       []model.Item{},
			"size":        size,
			"window":      window,
			"completed":   0,
			"remaining":   0,
			"total":       0,
			"source_pool": 0,
		}
		if h.cache != nil {
			if err := h.cache.SetJSON(r.Context(), cacheKey, out, focusQueueCacheTTL); err != nil {
				incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.error")
				log.Printf("focus-queue cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
			}
		}
		writeJSON(w, out)
		return
	}
	affinity := map[string]float64{}
	if h.sourceRepo != nil {
		if sources, e := h.sourceRepo.RecommendedByUser(r.Context(), userID, 30); e == nil {
			for _, s := range sources {
				affinity[s.SourceID] = s.AffinityScore
			}
		}
	}
	items := make([]model.Item, len(resp.Items))
	copy(items, resp.Items)
	sort.SliceStable(items, func(i, j int) bool {
		ai := affinity[items[i].SourceID]
		aj := affinity[items[j].SourceID]
		if ai != aj {
			return ai > aj
		}
		si := 0.0
		sj := 0.0
		if items[i].SummaryScore != nil {
			si = *items[i].SummaryScore
		}
		if items[j].SummaryScore != nil {
			sj = *items[j].SummaryScore
		}
		if si != sj {
			return si > sj
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	completed := 0
	for _, it := range items {
		if it.IsRead {
			completed++
		}
	}
	out := map[string]any{
		"items":            items,
		"size":             size,
		"window":           resp.Window,
		"completed":        completed,
		"remaining":        len(items) - completed,
		"total":            len(items),
		"source_pool":      resp.SourcePoolCount,
		"diversify_topics": resp.DiversifyTopics,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, focusQueueCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.error")
			log.Printf("focus-queue cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func (h *ItemHandler) TriageAll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	cacheBust := q.Get("cache_bust") == "1"
	cacheKey := cacheKeyTriageAll(userID)
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_all.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_all.error")
			log.Printf("triage-all cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "triage_all.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "triage_all.bypass")
	}

	items := make([]model.Item, 0, 200)
	page := 1
	for page <= 100 {
		resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
			Page:         page,
			PageSize:     200,
			Sort:         "newest",
			UnreadOnly:   true,
			FavoriteOnly: false,
			LaterOnly:    false,
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if resp == nil || len(resp.Items) == 0 {
			break
		}
		items = append(items, resp.Items...)
		if !resp.HasNext {
			break
		}
		page++
	}
	out := map[string]any{
		"items":            items,
		"size":             len(items),
		"window":           "all",
		"completed":        0,
		"remaining":        len(items),
		"total":            len(items),
		"source_pool":      0,
		"diversify_topics": false,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, triageAllCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_all.error")
			log.Printf("triage-all cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func (h *ItemHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.detail.Get(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, item)
}

func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id, userID); err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ItemHandler) Related(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 6)
	if limit < 1 || limit > 20 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	cacheKey := cacheKeyRelated(userID, id, limit)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "related.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "related.error")
			log.Printf("related cache get failed user_id=%s item_id=%s key=%s err=%v", userID, id, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "related.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "related.bypass")
	}

	var targetTopics []string
	if detail, err := h.detail.Get(r.Context(), id, userID); err == nil && detail != nil && detail.Summary != nil {
		targetTopics = detail.Summary.Topics
	}
	items, err := h.repo.ListRelated(r.Context(), id, userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	items = rerankAndFilterRelated(items, targetTopics, limit)
	annotateRelatedReasons(items, targetTopics)
	clusters := clusterRelatedItems(items)
	out := map[string]any{
		"items":    items,
		"clusters": clusters,
		"limit":    limit,
		"item_id":  id,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, relatedItemsCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "related.error")
			log.Printf("related cache set failed user_id=%s item_id=%s key=%s err=%v", userID, id, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func rerankAndFilterRelated(items []model.RelatedItem, targetTopics []string, limit int) []model.RelatedItem {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	targetSet := map[string]struct{}{}
	for _, t := range targetTopics {
		v := strings.TrimSpace(t)
		if v == "" {
			continue
		}
		targetSet[v] = struct{}{}
	}
	type scoredItem struct {
		item    model.RelatedItem
		score   float64
		overlap int
	}
	scored := make([]scoredItem, 0, len(items))
	for _, it := range items {
		overlap := 0
		if len(targetSet) > 0 {
			for _, topic := range it.Topics {
				if _, ok := targetSet[strings.TrimSpace(topic)]; ok {
					overlap++
				}
			}
		}
		// Hard filter to cut obvious noise while avoiding "no related items".
		if overlap == 0 && it.Similarity < 0.58 {
			continue
		}
		if overlap > 0 && it.Similarity < 0.42 {
			continue
		}
		overlapBoost := 0.0
		if overlap > 0 {
			overlapBoost = float64(overlap)
			if overlapBoost > 3 {
				overlapBoost = 3
			}
			overlapBoost *= 0.06
		}
		score := it.Similarity + overlapBoost
		scored = append(scored, scoredItem{item: it, score: score, overlap: overlap})
	}
	if len(scored) == 0 {
		// Fallback 1: keep reasonably high-similarity items.
		for _, it := range items {
			if it.Similarity >= 0.62 {
				scored = append(scored, scoredItem{item: it, score: it.Similarity, overlap: 0})
			}
		}
	}
	if len(scored) == 0 {
		// Fallback 2: at least return stronger half of candidates.
		for _, it := range items {
			if it.Similarity >= 0.50 {
				scored = append(scored, scoredItem{item: it, score: it.Similarity, overlap: 0})
			}
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].overlap != scored[j].overlap {
			return scored[i].overlap > scored[j].overlap
		}
		if scored[i].item.Similarity != scored[j].item.Similarity {
			return scored[i].item.Similarity > scored[j].item.Similarity
		}
		return scored[i].item.CreatedAt.After(scored[j].item.CreatedAt)
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]model.RelatedItem, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.item)
	}
	return out
}

func annotateRelatedReasons(items []model.RelatedItem, targetTopics []string) {
	targetSet := map[string]struct{}{}
	for _, t := range targetTopics {
		v := strings.TrimSpace(t)
		if v == "" {
			continue
		}
		targetSet[v] = struct{}{}
	}
	for i := range items {
		var shared []string
		for _, t := range items[i].Topics {
			if _, ok := targetSet[t]; ok {
				shared = append(shared, t)
				if len(shared) >= 3 {
					break
				}
			}
		}
		items[i].ReasonTopics = shared
		if len(shared) > 0 {
			reason := fmt.Sprintf("shared topics: %s", strings.Join(shared, ", "))
			items[i].Reason = &reason
			continue
		}
		switch {
		case items[i].Similarity >= 0.8:
			reason := "very high semantic similarity"
			items[i].Reason = &reason
		case items[i].Similarity >= 0.65:
			reason := "high semantic similarity"
			items[i].Reason = &reason
		default:
			reason := "semantic similarity match"
			items[i].Reason = &reason
		}
	}
}

type relatedClusterResponse struct {
	ID             string              `json:"id"`
	Label          string              `json:"label"`
	Size           int                 `json:"size"`
	MaxSimilarity  float64             `json:"max_similarity"`
	Representative model.RelatedItem   `json:"representative"`
	Items          []model.RelatedItem `json:"items"`
}

func clusterRelatedItems(items []model.RelatedItem) []relatedClusterResponse {
	if len(items) == 0 {
		return nil
	}
	remaining := make([]model.RelatedItem, len(items))
	copy(remaining, items)
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Similarity != remaining[j].Similarity {
			return remaining[i].Similarity > remaining[j].Similarity
		}
		return remaining[i].CreatedAt.After(remaining[j].CreatedAt)
	})

	used := make([]bool, len(remaining))
	clusters := make([]relatedClusterResponse, 0, len(remaining))
	for i := range remaining {
		if used[i] {
			continue
		}
		seed := remaining[i]
		used[i] = true
		members := []model.RelatedItem{seed}
		maxSim := seed.Similarity
		seedTopicSet := map[string]struct{}{}
		for _, t := range seed.Topics {
			if t != "" {
				seedTopicSet[t] = struct{}{}
			}
		}
		for j := i + 1; j < len(remaining); j++ {
			if used[j] {
				continue
			}
			cand := remaining[j]
			if shouldClusterRelated(seed, seedTopicSet, cand) {
				used[j] = true
				members = append(members, cand)
				if cand.Similarity > maxSim {
					maxSim = cand.Similarity
				}
			}
		}
		sort.SliceStable(members, func(a, b int) bool {
			if members[a].Similarity != members[b].Similarity {
				return members[a].Similarity > members[b].Similarity
			}
			return members[a].CreatedAt.After(members[b].CreatedAt)
		})
		label := clusterLabel(members[0])
		clusters = append(clusters, relatedClusterResponse{
			ID:             members[0].ID,
			Label:          label,
			Size:           len(members),
			MaxSimilarity:  maxSim,
			Representative: members[0],
			Items:          members,
		})
	}
	return clusters
}

func shouldClusterRelated(seed model.RelatedItem, seedTopics map[string]struct{}, cand model.RelatedItem) bool {
	// Strong similarity alone groups items.
	if cand.Similarity >= 0.78 {
		return true
	}
	// Otherwise require moderate similarity + topic overlap.
	if cand.Similarity < 0.58 {
		return false
	}
	if len(seedTopics) == 0 || len(cand.Topics) == 0 {
		return false
	}
	for _, t := range cand.Topics {
		if _, ok := seedTopics[t]; ok {
			return true
		}
	}
	return false
}

func clusterLabel(it model.RelatedItem) string {
	if len(it.Topics) > 0 && it.Topics[0] != "" {
		return it.Topics[0]
	}
	if it.Title != nil && *it.Title != "" {
		return *it.Title
	}
	return "Related"
}

func (h *ItemHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	inserted, err := h.repo.MarkRead(r.Context(), userID, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if inserted && h.streakRepo != nil {
		_ = h.streakRepo.IncrementRead(r.Context(), userID, timeutil.NowJST(), 3)
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_read": true})
}

func (h *ItemHandler) MarkUnread(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkUnread(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_read": false})
}

func (h *ItemHandler) MarkReadBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		ItemIDs       []string `json:"item_ids"`
		Status        *string  `json:"status"`
		SourceID      *string  `json:"source_id"`
		Topic         *string  `json:"topic"`
		UnreadOnly    bool     `json:"unread_only"`
		ReadOnly      bool     `json:"read_only"`
		FavoriteOnly  bool     `json:"favorite_only"`
		LaterOnly     bool     `json:"later_only"`
		OlderThanDays *int     `json:"older_than_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if len(body.ItemIDs) > 0 {
		if len(body.ItemIDs) > 100 {
			http.Error(w, "too many item_ids", http.StatusBadRequest)
			return
		}
		updated, err := h.repo.MarkReadBulkByIDs(r.Context(), userID, body.ItemIDs)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		h.invalidateUserCaches(r.Context(), userID)
		writeJSON(w, map[string]any{"status": "ok", "updated_count": updated})
		return
	}
	if body.UnreadOnly && body.ReadOnly {
		http.Error(w, "unread_only and read_only cannot both be true", http.StatusBadRequest)
		return
	}
	updated, err := h.repo.MarkReadBulk(r.Context(), userID, repository.BulkMarkReadParams{
		Status:        body.Status,
		SourceID:      body.SourceID,
		Topic:         body.Topic,
		UnreadOnly:    body.UnreadOnly,
		ReadOnly:      body.ReadOnly,
		FavoriteOnly:  body.FavoriteOnly,
		LaterOnly:     body.LaterOnly,
		OlderThanDays: body.OlderThanDays,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"status": "ok", "updated_count": updated})
}

func (h *ItemHandler) MarkLater(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkLater(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_later": true})
}

func (h *ItemHandler) UnmarkLater(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.UnmarkLater(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_later": false})
}

func (h *ItemHandler) MarkLaterBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if len(body.ItemIDs) == 0 {
		http.Error(w, "item_ids is required", http.StatusBadRequest)
		return
	}
	if len(body.ItemIDs) > 100 {
		http.Error(w, "too many item_ids", http.StatusBadRequest)
		return
	}
	updated, err := h.repo.MarkLaterBulk(r.Context(), userID, body.ItemIDs)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"status": "ok", "updated_count": updated})
}

func (h *ItemHandler) SetFeedback(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	var body struct {
		Rating     int  `json:"rating"`
		IsFavorite bool `json:"is_favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Rating < -1 || body.Rating > 1 {
		http.Error(w, "invalid rating", http.StatusBadRequest)
		return
	}
	fb, err := h.repo.UpsertFeedback(r.Context(), userID, id, body.Rating, body.IsFavorite)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, fb)
}

func (h *ItemHandler) Retry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetForRetry(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	summaryEmpty := item.Summary == nil || strings.TrimSpace(*item.Summary) == ""
	retryable := item.Status == "failed" || item.Status == "fetched" || item.Status == "facts_extracted" || item.Status == "summarized"
	if !retryable && !(item.Status == "summarized" && summaryEmpty) {
		http.Error(w, "item is not retryable", http.StatusConflict)
		return
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}
	if err := h.publisher.SendItemCreatedE(r.Context(), item.ID, item.SourceID, item.URL); err != nil {
		http.Error(w, "failed to enqueue retry", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":  "queued",
		"item_id": item.ID,
	})
}

func (h *ItemHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	var sourceID *string
	if v := q.Get("source_id"); v != "" {
		sourceID = &v
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}

	items, err := h.repo.ListFailedForRetry(r.Context(), userID, sourceID)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	queued := 0
	failed := 0
	for _, item := range items {
		if err := h.publisher.SendItemCreatedE(r.Context(), item.ID, item.SourceID, item.URL); err != nil {
			failed++
			continue
		}
		queued++
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{
		"status":       "queued",
		"source_id":    sourceID,
		"matched":      len(items),
		"queued_count": queued,
		"failed_count": failed,
	})
}
