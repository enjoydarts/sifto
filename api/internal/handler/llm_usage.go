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
	return &LLMUsageHandler{usage: service.NewLLMUsageService(repo, executionRepo, nil), cache: cache}
}

const (
	llmUsageListCacheTTL                = 2 * time.Minute
	llmUsageDailySummaryCacheTTL        = 5 * time.Minute
	llmUsageModelSummaryCacheTTL        = 5 * time.Minute
	llmUsageCurrentMonthSummaryCacheTTL = 10 * time.Minute
)

func NewLLMUsageHandlerWithValueMetrics(repo *repository.LLMUsageLogRepo, executionRepo *repository.LLMExecutionEventRepo, valueRepo *repository.LLMValueMetricsRepo, cache service.JSONCache) *LLMUsageHandler {
	return &LLMUsageHandler{usage: service.NewLLMUsageService(repo, executionRepo, valueRepo), cache: cache}
}

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

func parseUsageMonthJST(r *http.Request) (time.Time, string, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("month"))
	if raw == "" {
		now := time.Now().In(time.FixedZone("JST", 9*60*60))
		month := fmt.Sprintf("%04d-%02d", now.Year(), now.Month())
		return now, month, true
	}
	parsed, err := repository.ParseMonthJST(raw)
	if err != nil {
		return time.Time{}, "", false
	}
	return parsed, parsed.Format("2006-01"), true
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
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageListCacheTTL, func() ([]service.LLMUsageLogView, error) {
		return h.usage.List(r.Context(), userID, limit)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
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
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageDailySummaryVersioned(userID, 0, days))
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageDailySummaryCacheTTL, func() ([]service.LLMUsageDailySummaryView, error) {
		return h.usage.DailySummary(r.Context(), userID, days)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
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
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageModelSummaryVersioned(userID, 0, days))
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageModelSummaryCacheTTL, func() ([]service.LLMUsageModelSummaryView, error) {
		return h.usage.ModelSummary(r.Context(), userID, days)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
		return
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
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageModelSummaryCacheTTL, func() ([]service.LLMUsageAnalysisSummaryView, error) {
		return h.usage.AnalysisSummary(r.Context(), userID, days)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ProviderSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	monthTime, monthKey, ok := parseUsageMonthJST(r)
	if !ok {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageProviderCurrentMonthVersioned(userID, 0, monthKey))
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageCurrentMonthSummaryCacheTTL, func() ([]service.LLMUsageProviderMonthSummaryView, error) {
		return h.usage.ProviderSummaryMonth(r.Context(), userID, monthTime)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) PurposeSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	monthTime, monthKey, ok := parseUsageMonthJST(r)
	if !ok {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsagePurposeCurrentMonthVersioned(userID, 0, monthKey))
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageCurrentMonthSummaryCacheTTL, func() ([]service.LLMUsagePurposeMonthSummaryView, error) {
		return h.usage.PurposeSummaryMonth(r.Context(), userID, monthTime)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ExecutionSummaryCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	daysRaw := strings.TrimSpace(r.URL.Query().Get("days"))
	if daysRaw != "" {
		days, ok := parseUsageDays(r)
		if !ok {
			http.Error(w, "invalid days", http.StatusBadRequest)
			return
		}
		cacheBust := r.URL.Query().Get("cache_bust") == "1"
		cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageExecutionSummaryVersioned(userID, 0, days))
		rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageModelSummaryCacheTTL, func() ([]service.LLMExecutionCurrentMonthSummaryView, error) {
			return h.usage.ExecutionSummary(r.Context(), userID, days)
		}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
		if fetchErr != nil {
			writeRepoError(w, fetchErr)
			return
		}
		writeJSON(w, rows)
		return
	}
	monthTime, monthKey, ok := parseUsageMonthJST(r)
	if !ok {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageExecutionCurrentMonthVersioned(userID, 0, monthKey))
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageCurrentMonthSummaryCacheTTL, func() ([]service.LLMExecutionCurrentMonthSummaryView, error) {
		return h.usage.ExecutionSummaryMonth(r.Context(), userID, monthTime)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
		return
	}
	writeJSON(w, rows)
}

func (h *LLMUsageHandler) ValueMetricsCurrentMonth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	monthTime, monthKey, ok := parseUsageMonthJST(r)
	if !ok {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	cacheKey, err := h.llmUsageCacheKey(r.Context(), userID, cacheKeyLLMUsageValueMetricsCurrentMonthVersioned(userID, 0, monthKey))
	rows, fetchErr := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, llmUsageCurrentMonthSummaryCacheTTL, func() ([]service.LLMValueMetricView, error) {
		return h.usage.ValueMetricsMonth(r.Context(), userID, monthTime)
	}, cacheFetchOptions{cacheBust: cacheBust, cacheKeyErr: err})
	if fetchErr != nil {
		writeRepoError(w, fetchErr)
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
