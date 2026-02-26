package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
)

type ItemHandler struct {
	repo      *repository.ItemRepo
	publisher *service.EventPublisher
	cache     service.JSONCache
}

func NewItemHandler(repo *repository.ItemRepo, publisher *service.EventPublisher, cache service.JSONCache) *ItemHandler {
	return &ItemHandler{repo: repo, publisher: publisher, cache: cache}
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
	favoriteOnly := q.Get("favorite_only") == "true"
	resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
		Status:       status,
		SourceID:     sourceID,
		Topic:        topic,
		UnreadOnly:   unreadOnly,
		FavoriteOnly: favoriteOnly,
		Sort:         sort,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeRepoError(w, err)
		return
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

func (h *ItemHandler) ReadingPlan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	window := q.Get("window")
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
	}
	cacheKey := fmt.Sprintf("readingplan:%s:%s:%d:%t:%t", userID, params.Window, params.Size, params.DiversifyTopics, params.ExcludeRead)
	cacheBust := q.Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.ReadingPlanResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			readingPlanCacheCounter.hits.Add(1)
			log.Printf("reading-plan cache hit user_id=%s key=%s", userID, cacheKey)
			writeJSON(w, &cached)
			return
		} else if err != nil {
			readingPlanCacheCounter.errors.Add(1)
			log.Printf("reading-plan cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		readingPlanCacheCounter.misses.Add(1)
		log.Printf("reading-plan cache miss user_id=%s key=%s", userID, cacheKey)
	} else if cacheBust {
		readingPlanCacheCounter.bypass.Add(1)
		log.Printf("reading-plan cache bypass user_id=%s key=%s", userID, cacheKey)
	}

	resp, err := h.repo.ReadingPlan(r.Context(), userID, params)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.cache != nil && resp != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, 45*time.Second); err != nil {
			readingPlanCacheCounter.errors.Add(1)
			log.Printf("reading-plan cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetDetail(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, item)
}

func (h *ItemHandler) Related(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 6)
	if limit < 1 || limit > 20 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	items, err := h.repo.ListRelated(r.Context(), id, userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	clusters := clusterRelatedItems(items)
	writeJSON(w, map[string]any{
		"items":    items,
		"clusters": clusters,
		"limit":    limit,
		"item_id":  id,
	})
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
	if err := h.repo.MarkRead(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"item_id": id, "is_read": true})
}

func (h *ItemHandler) MarkUnread(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkUnread(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"item_id": id, "is_read": false})
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
	if item.Status != "failed" {
		http.Error(w, "item is not failed", http.StatusConflict)
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
