package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

type SettingsHandler struct {
	repo        *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cipher      *service.SecretCipher
}

func NewSettingsHandler(repo *repository.UserSettingsRepo, llmUsageRepo *repository.LLMUsageLogRepo, cipher *service.SecretCipher) *SettingsHandler {
	return &SettingsHandler{repo: repo, llmUsageRepo: llmUsageRepo, cipher: cipher}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	settings, err := h.repo.EnsureDefaults(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	nowJST := timeutil.NowJST()
	monthStart := time.Date(nowJST.Year(), nowJST.Month(), 1, 0, 0, 0, 0, timeutil.JST)
	nextMonth := monthStart.AddDate(0, 1, 0)
	usedCostUSD, err := h.llmUsageRepo.SumEstimatedCostByUserBetween(r.Context(), userID, monthStart, nextMonth)
	if err != nil {
		http.Error(w, "failed to load usage summary", http.StatusInternalServerError)
		return
	}

	var remainingBudgetUSD *float64
	var remainingPct *float64
	if settings.MonthlyBudgetUSD != nil && *settings.MonthlyBudgetUSD > 0 {
		v := *settings.MonthlyBudgetUSD - usedCostUSD
		remainingBudgetUSD = &v
		p := (v / *settings.MonthlyBudgetUSD) * 100
		remainingPct = &p
	}

	writeJSON(w, map[string]any{
		"user_id":                   settings.UserID,
		"has_anthropic_api_key":     settings.HasAnthropicAPIKey,
		"anthropic_api_key_last4":   settings.AnthropicAPIKeyLast4,
		"monthly_budget_usd":        settings.MonthlyBudgetUSD,
		"budget_alert_enabled":      settings.BudgetAlertEnabled,
		"budget_alert_threshold_pct": settings.BudgetAlertThresholdPct,
		"current_month": map[string]any{
			"month_jst":             monthStart.Format("2006-01"),
			"period_start_jst":      monthStart.Format(time.RFC3339),
			"period_end_jst":        nextMonth.Format(time.RFC3339),
			"estimated_cost_usd":    usedCostUSD,
			"remaining_budget_usd":  remainingBudgetUSD,
			"remaining_budget_pct":  remainingPct,
		},
	})
}

func (h *SettingsHandler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		MonthlyBudgetUSD       *float64 `json:"monthly_budget_usd"`
		BudgetAlertEnabled     bool     `json:"budget_alert_enabled"`
		BudgetAlertThresholdPct int     `json:"budget_alert_threshold_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.BudgetAlertThresholdPct < 1 || body.BudgetAlertThresholdPct > 99 {
		http.Error(w, "invalid budget_alert_threshold_pct", http.StatusBadRequest)
		return
	}
	if body.MonthlyBudgetUSD != nil && *body.MonthlyBudgetUSD < 0 {
		http.Error(w, "invalid monthly_budget_usd", http.StatusBadRequest)
		return
	}
	var budget *float64
	if body.MonthlyBudgetUSD != nil && *body.MonthlyBudgetUSD > 0 {
		budget = body.MonthlyBudgetUSD
	}
	settings, err := h.repo.UpsertBudgetConfig(r.Context(), userID, budget, body.BudgetAlertEnabled, body.BudgetAlertThresholdPct)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, settings)
}

func (h *SettingsHandler) SetAnthropicAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(body.APIKey)
	if key == "" {
		http.Error(w, "api_key is required", http.StatusBadRequest)
		return
	}
	if h.cipher == nil || !h.cipher.Enabled() {
		http.Error(w, "user secret encryption is not configured", http.StatusInternalServerError)
		return
	}
	enc, err := h.cipher.EncryptString(key)
	if err != nil {
		http.Error(w, "failed to encrypt api key", http.StatusInternalServerError)
		return
	}
	last4 := key
	if len(last4) > 4 {
		last4 = last4[len(last4)-4:]
	}
	settings, err := h.repo.SetAnthropicAPIKey(r.Context(), userID, enc, last4)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id":                 settings.UserID,
		"has_anthropic_api_key":   settings.HasAnthropicAPIKey,
		"anthropic_api_key_last4": settings.AnthropicAPIKeyLast4,
	})
}

func (h *SettingsHandler) DeleteAnthropicAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	settings, err := h.repo.ClearAnthropicAPIKey(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id":                 settings.UserID,
		"has_anthropic_api_key":   settings.HasAnthropicAPIKey,
		"anthropic_api_key_last4": settings.AnthropicAPIKeyLast4,
	})
}
