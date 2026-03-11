package handler

import (
	"net/http"
	"strconv"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type LLMUsageHandler struct {
	usage *service.LLMUsageService
}

func NewLLMUsageHandler(repo *repository.LLMUsageLogRepo, executionRepo *repository.LLMExecutionEventRepo) *LLMUsageHandler {
	return &LLMUsageHandler{usage: service.NewLLMUsageService(repo, executionRepo)}
}

func parseUsageLimit(r *http.Request) (int, bool) {
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 100)
	if limit < 1 || limit > 500 {
		return 0, false
	}
	return limit, true
}

func parseUsageDays(r *http.Request) (int, bool) {
	days := parseIntOrDefault(r.URL.Query().Get("days"), 14)
	if days < 1 || days > 365 {
		return 0, false
	}
	return days, true
}

func (h *LLMUsageHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit, ok := parseUsageLimit(r)
	if !ok {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.usage.List(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) DailySummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days, ok := parseUsageDays(r)
	if !ok {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	rows, err := h.usage.DailySummary(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ModelSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days, ok := parseUsageDays(r)
	if !ok {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	rows, err := h.usage.ModelSummary(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ProviderSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.usage.ProviderSummaryCurrentMonth(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) PurposeSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.usage.PurposeSummaryCurrentMonth(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ExecutionSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.usage.ExecutionSummaryCurrentMonth(r.Context(), userID)
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
