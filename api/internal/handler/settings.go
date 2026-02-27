package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

type SettingsHandler struct {
	repo         *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	cipher       *service.SecretCipher
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
		"user_id":                    settings.UserID,
		"has_anthropic_api_key":      settings.HasAnthropicAPIKey,
		"anthropic_api_key_last4":    settings.AnthropicAPIKeyLast4,
		"has_openai_api_key":         settings.HasOpenAIAPIKey,
		"openai_api_key_last4":       settings.OpenAIAPIKeyLast4,
		"has_inoreader_oauth":        settings.HasInoreaderOAuth,
		"inoreader_token_expires_at": settings.InoreaderTokenExpiresAt,
		"monthly_budget_usd":         settings.MonthlyBudgetUSD,
		"budget_alert_enabled":       settings.BudgetAlertEnabled,
		"budget_alert_threshold_pct": settings.BudgetAlertThresholdPct,
		"digest_email_enabled":       settings.DigestEmailEnabled,
		"reading_plan": map[string]any{
			"window":           settings.ReadingPlanWindow,
			"size":             settings.ReadingPlanSize,
			"diversify_topics": settings.ReadingPlanDiversifyTopics,
			"exclude_read":     settings.ReadingPlanExcludeRead,
		},
		"llm_models": map[string]any{
			"anthropic_facts":             settings.AnthropicFactsModel,
			"anthropic_summary":           settings.AnthropicSummaryModel,
			"anthropic_digest_cluster":    settings.AnthropicDigestClusterModel,
			"anthropic_digest":            settings.AnthropicDigestModel,
			"anthropic_source_suggestion": settings.AnthropicSourceSuggestModel,
			"openai_embedding":            settings.OpenAIEmbeddingModel,
		},
		"current_month": map[string]any{
			"month_jst":            monthStart.Format("2006-01"),
			"period_start_jst":     monthStart.Format(time.RFC3339),
			"period_end_jst":       nextMonth.Format(time.RFC3339),
			"estimated_cost_usd":   usedCostUSD,
			"remaining_budget_usd": remainingBudgetUSD,
			"remaining_budget_pct": remainingPct,
		},
	})
}

func oauthRedirectURIFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(os.Getenv("INOREADER_OAUTH_REDIRECT_URI")); v != "" {
		return v
	}
	scheme := "https"
	if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xf != "" {
		scheme = xf
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s/api/settings/inoreader/callback", scheme, host)
}

func randomOAuthState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *SettingsHandler) InoreaderConnect(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(os.Getenv("INOREADER_CLIENT_ID")) == "" || strings.TrimSpace(os.Getenv("INOREADER_CLIENT_SECRET")) == "" {
		http.Error(w, "inoreader oauth is not configured", http.StatusInternalServerError)
		return
	}
	state, err := randomOAuthState()
	if err != nil {
		http.Error(w, "failed to build oauth state", http.StatusInternalServerError)
		return
	}
	redirectURI := oauthRedirectURIFromRequest(r)
	q := url.Values{}
	q.Set("client_id", strings.TrimSpace(os.Getenv("INOREADER_CLIENT_ID")))
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "read")
	q.Set("state", state)
	connectURL := "https://www.inoreader.com/oauth2/auth?" + q.Encode()
	http.SetCookie(w, &http.Cookie{
		Name:     "inoreader_oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   10 * 60,
	})
	http.Redirect(w, r, connectURL, http.StatusFound)
}

func (h *SettingsHandler) InoreaderCallback(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Redirect(w, r, "/settings?inoreader=error&reason=missing_code", http.StatusFound)
		return
	}
	stateCookie, err := r.Cookie("inoreader_oauth_state")
	if err != nil || strings.TrimSpace(stateCookie.Value) == "" || stateCookie.Value != state {
		http.Redirect(w, r, "/settings?inoreader=error&reason=invalid_state", http.StatusFound)
		return
	}
	redirectURI := oauthRedirectURIFromRequest(r)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", strings.TrimSpace(os.Getenv("INOREADER_CLIENT_ID")))
	form.Set("client_secret", strings.TrimSpace(os.Getenv("INOREADER_CLIENT_SECRET")))

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://www.inoreader.com/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		http.Redirect(w, r, "/settings?inoreader=error&reason=token_request", http.StatusFound)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		http.Redirect(w, r, "/settings?inoreader=error&reason=token_exchange", http.StatusFound)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Redirect(w, r, "/settings?inoreader=error&reason=token_status", http.StatusFound)
		return
	}
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil || strings.TrimSpace(tokenResp.AccessToken) == "" {
		http.Redirect(w, r, "/settings?inoreader=error&reason=token_parse", http.StatusFound)
		return
	}
	if h.cipher == nil || !h.cipher.Enabled() {
		http.Redirect(w, r, "/settings?inoreader=error&reason=cipher", http.StatusFound)
		return
	}
	accessEnc, err := h.cipher.EncryptString(tokenResp.AccessToken)
	if err != nil {
		http.Redirect(w, r, "/settings?inoreader=error&reason=encrypt_access", http.StatusFound)
		return
	}
	var refreshEnc *string
	if strings.TrimSpace(tokenResp.RefreshToken) != "" {
		v, err := h.cipher.EncryptString(tokenResp.RefreshToken)
		if err != nil {
			http.Redirect(w, r, "/settings?inoreader=error&reason=encrypt_refresh", http.StatusFound)
			return
		}
		refreshEnc = &v
	}
	var expiresAt *time.Time
	if tokenResp.ExpiresIn > 0 {
		v := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		expiresAt = &v
	}
	if _, err := h.repo.SetInoreaderOAuthTokens(r.Context(), userID, accessEnc, refreshEnc, expiresAt); err != nil {
		http.Redirect(w, r, "/settings?inoreader=error&reason=save", http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "inoreader_oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/settings?inoreader=connected", http.StatusFound)
}

func (h *SettingsHandler) DeleteInoreaderOAuth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	settings, err := h.repo.ClearInoreaderOAuthTokens(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id":                 settings.UserID,
		"has_inoreader_oauth":     settings.HasInoreaderOAuth,
		"inoreader_token_expires": settings.InoreaderTokenExpiresAt,
	})
}

func (h *SettingsHandler) UpdateLLMModels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		AnthropicFacts            *string `json:"anthropic_facts"`
		AnthropicSummary          *string `json:"anthropic_summary"`
		AnthropicDigestCluster    *string `json:"anthropic_digest_cluster"`
		AnthropicDigest           *string `json:"anthropic_digest"`
		AnthropicSourceSuggestion *string `json:"anthropic_source_suggestion"`
		OpenAIEmbedding           *string `json:"openai_embedding"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	norm := func(v *string) *string {
		if v == nil {
			return nil
		}
		s := strings.TrimSpace(*v)
		if s == "" {
			return nil
		}
		return &s
	}
	openAIEmbedding := norm(body.OpenAIEmbedding)
	if openAIEmbedding != nil && !service.IsSupportedOpenAIEmbeddingModel(*openAIEmbedding) {
		http.Error(w, "invalid openai_embedding model", http.StatusBadRequest)
		return
	}
	settings, err := h.repo.UpsertLLMModelConfig(
		r.Context(),
		userID,
		norm(body.AnthropicFacts),
		norm(body.AnthropicSummary),
		norm(body.AnthropicDigestCluster),
		norm(body.AnthropicDigest),
		norm(body.AnthropicSourceSuggestion),
		openAIEmbedding,
	)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id": settings.UserID,
		"llm_models": map[string]any{
			"anthropic_facts":             settings.AnthropicFactsModel,
			"anthropic_summary":           settings.AnthropicSummaryModel,
			"anthropic_digest_cluster":    settings.AnthropicDigestClusterModel,
			"anthropic_digest":            settings.AnthropicDigestModel,
			"anthropic_source_suggestion": settings.AnthropicSourceSuggestModel,
			"openai_embedding":            settings.OpenAIEmbeddingModel,
		},
	})
}

func (h *SettingsHandler) UpdateReadingPlan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Window          string `json:"window"`
		Size            int    `json:"size"`
		DiversifyTopics bool   `json:"diversify_topics"`
		ExcludeRead     bool   `json:"exclude_read"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Window != "24h" && body.Window != "today_jst" && body.Window != "7d" {
		http.Error(w, "invalid window", http.StatusBadRequest)
		return
	}
	if body.Size < 1 || body.Size > 100 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	settings, err := h.repo.UpsertReadingPlanConfig(r.Context(), userID, body.Window, body.Size, body.DiversifyTopics, body.ExcludeRead)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id": settings.UserID,
		"reading_plan": map[string]any{
			"window":           settings.ReadingPlanWindow,
			"size":             settings.ReadingPlanSize,
			"diversify_topics": settings.ReadingPlanDiversifyTopics,
			"exclude_read":     settings.ReadingPlanExcludeRead,
		},
	})
}

func (h *SettingsHandler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		MonthlyBudgetUSD        *float64 `json:"monthly_budget_usd"`
		BudgetAlertEnabled      bool     `json:"budget_alert_enabled"`
		BudgetAlertThresholdPct int      `json:"budget_alert_threshold_pct"`
		DigestEmailEnabled      bool     `json:"digest_email_enabled"`
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
	settings, err := h.repo.UpsertBudgetConfig(r.Context(), userID, budget, body.BudgetAlertEnabled, body.BudgetAlertThresholdPct, body.DigestEmailEnabled)
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

func (h *SettingsHandler) SetOpenAIAPIKey(w http.ResponseWriter, r *http.Request) {
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
	settings, err := h.repo.SetOpenAIAPIKey(r.Context(), userID, enc, last4)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id":              settings.UserID,
		"has_openai_api_key":   settings.HasOpenAIAPIKey,
		"openai_api_key_last4": settings.OpenAIAPIKeyLast4,
	})
}

func (h *SettingsHandler) DeleteOpenAIAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	settings, err := h.repo.ClearOpenAIAPIKey(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id":              settings.UserID,
		"has_openai_api_key":   settings.HasOpenAIAPIKey,
		"openai_api_key_last4": settings.OpenAIAPIKeyLast4,
	})
}
