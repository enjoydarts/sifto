package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
)

type ItemHandler struct {
	repo      *repository.ItemRepo
	publisher *service.EventPublisher
}

func NewItemHandler(repo *repository.ItemRepo, publisher *service.EventPublisher) *ItemHandler {
	return &ItemHandler{repo: repo, publisher: publisher}
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	var status, sourceID *string
	if v := q.Get("status"); v != "" {
		status = &v
	}
	if v := q.Get("source_id"); v != "" {
		sourceID = &v
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
	resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
		Status:    status,
		SourceID:  sourceID,
		UnreadOnly: unreadOnly,
		Sort:      sort,
		Page:      page,
		PageSize:  pageSize,
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
	resp, err := h.repo.ReadingPlan(r.Context(), userID, repository.ReadingPlanParams{
		Window:          window,
		Size:            size,
		DiversifyTopics: diversify,
		ExcludeRead:     excludeRead,
	})
	if err != nil {
		writeRepoError(w, err)
		return
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
