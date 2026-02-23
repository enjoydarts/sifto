package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
)

type ItemHandler struct{ repo *repository.ItemRepo }

func NewItemHandler(repo *repository.ItemRepo) *ItemHandler { return &ItemHandler{repo} }

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
	items, err := h.repo.List(r.Context(), userID, status, sourceID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, items)
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
