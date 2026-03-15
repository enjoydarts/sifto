package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type LLMUsageHandler struct {
	usage *service.LLMUsageService
	cache service.JSONCache
}

func NewLLMUsageHandler(repo *repository.LLMUsageLogRepo, executionRepo *repository.LLMExecutionEventRepo, cache service.JSONCache) *LLMUsageHandler {
	return &LLMUsageHandler{usage: service.NewLLMUsageService(repo, executionRepo), cache: cache}
}

const (
	llmUsageListCacheTTL                = 2 * time.Minute
	llmUsageDailySummaryCacheTTL        = 5 * time.Minute
	llmUsageModelSummaryCacheTTL        = 5 * time.Minute
	llmUsageCurrentMonthSummaryCacheTTL = 10 * time.Minute
)

func (h *LLMUsageHandler) llmUsageCacheKey(ctx context.Context, userID, fallbackKey string) (string, error) {
	version := int64(0)
	if h.cache != nil {
		var err error
		version, err = h.cache.GetVersion(ctx, cacheVersionKeyUserLLMUsage(userID))
		if err != nil {
			return "", err
		}
	}
	return strings.Replace(fallbackKey, ":v=0", fmt.Sprintf(":v=%d", version), 1), nil
}

func (h *LLMUsageHandler) bumpUserLLMUsageVersion(ctx context.Context, userID string) error {
	if h.cache == nil || userID == "" {
		return nil
	}
	_, err := h.cache.BumpVersion(ctx, cacheVersionKeyUserLLMUsage(userID))
	return err
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
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageListVersioned(userID, 0, limit))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMUsageLogView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.List(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageListCacheTTL)
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
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageDailySummaryVersioned(userID, 0, days))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMUsageDailySummaryView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.DailySummary(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageDailySummaryCacheTTL)
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
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageModelSummaryVersioned(userID, 0, days))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMUsageModelSummaryView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.ModelSummary(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageModelSummaryCacheTTL)
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) AnalysisSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days, ok := parseUsageDays(r)
	if !ok {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageAnalysisVersioned(userID, 0, days))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMUsageAnalysisSummaryView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.AnalysisSummary(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageModelSummaryCacheTTL)
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ProviderSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageProviderCurrentMonthVersioned(userID, 0))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMUsageProviderMonthSummaryView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.ProviderSummaryCurrentMonth(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageCurrentMonthSummaryCacheTTL)
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) PurposeSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsagePurposeCurrentMonthVersioned(userID, 0))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMUsagePurposeMonthSummaryView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.PurposeSummaryCurrentMonth(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageCurrentMonthSummaryCacheTTL)
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ExecutionSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageExecutionCurrentMonthVersioned(userID, 0))
	if err == nil && h.cache != nil && !cacheBust {
		var cached []service.LLMExecutionCurrentMonthSummaryView
		if ok, cacheErr := h.cache.GetJSON(r.Context(), cacheKey, &cached); cacheErr == nil && ok {
			writeJSON(w, cached)
			return
		}
	}
	rows, err := h.usage.ExecutionSummaryCurrentMonth(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err == nil && h.cache != nil {
		_ = h.cache.SetJSON(r.Context(), cacheKey, rows, llmUsageCurrentMonthSummaryCacheTTL)
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
