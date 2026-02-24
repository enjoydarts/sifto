package handler

import (
	"net/http"
	"strconv"

	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
)

type LLMUsageHandler struct{ repo *repository.LLMUsageLogRepo }

func NewLLMUsageHandler(repo *repository.LLMUsageLogRepo) *LLMUsageHandler {
	return &LLMUsageHandler{repo: repo}
}

func (h *LLMUsageHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 100)
	if limit < 1 || limit > 500 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.ListByUser(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) DailySummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 14)
	if days < 1 || days > 365 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.DailySummaryByUser(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func parseIntOrDefault(s string, d int) int {
	if s == "" {
		return d
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return v
}
