package handler

import (
	"net/http"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/repository"
)

type ProviderModelUpdateHandler struct {
	repo *repository.ProviderModelUpdateRepo
}

func NewProviderModelUpdateHandler(repo *repository.ProviderModelUpdateRepo) *ProviderModelUpdateHandler {
	return &ProviderModelUpdateHandler{repo: repo}
}

func (h *ProviderModelUpdateHandler) ListRecent(w http.ResponseWriter, r *http.Request) {
	days := parseIntOrDefault(r.URL.Query().Get("days"), 14)
	if days < 1 || days > 90 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 30)
	if limit < 1 || limit > 200 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	events, err := h.repo.ListRecent(r.Context(), since, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, events)
}
