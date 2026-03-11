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

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type SettingsHandler struct {
	repo     *repository.UserSettingsRepo
	cipher   *service.SecretCipher
	settings *service.SettingsService
}

func NewSettingsHandler(repo *repository.UserSettingsRepo, llmUsageRepo *repository.LLMUsageLogRepo, cipher *service.SecretCipher) *SettingsHandler {
	return &SettingsHandler{
		repo:     repo,
		cipher:   cipher,
		settings: service.NewSettingsService(repo, llmUsageRepo, cipher),
	}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	payload, err := h.settings.Get(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, payload)
}

func (h *SettingsHandler) GetLLMCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, service.LLMCatalogData())
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
		Facts             *string `json:"facts"`
		Summary           *string `json:"summary"`
		DigestCluster     *string `json:"digest_cluster"`
		Digest            *string `json:"digest"`
		Ask               *string `json:"ask"`
		SourceSuggestion  *string `json:"source_suggestion"`
		Embedding         *string `json:"embedding"`
		FactsCheck        *string `json:"facts_check"`
		FaithfulnessCheck *string `json:"faithfulness_check"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	settings, err := h.settings.UpdateLLMModels(r.Context(), userID, service.UpdateLLMModelsInput{
		Facts:             body.Facts,
		Summary:           body.Summary,
		DigestCluster:     body.DigestCluster,
		Digest:            body.Digest,
		Ask:               body.Ask,
		SourceSuggestion:  body.SourceSuggestion,
		Embedding:         body.Embedding,
		FactsCheck:        body.FactsCheck,
		FaithfulnessCheck: body.FaithfulnessCheck,
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "invalid model for ") || err.Error() == "invalid embedding model" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"user_id":    settings.UserID,
		"llm_models": service.LLMModelSettingsPayload(settings),
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
	settings, err := h.settings.UpdateReadingPlan(r.Context(), userID, body.Window, body.Size, body.DiversifyTopics, body.ExcludeRead)
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
	settings, err := h.settings.UpdateBudget(r.Context(), userID, budget, body.BudgetAlertEnabled, body.BudgetAlertThresholdPct, body.DigestEmailEnabled)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, settings)
}

func (h *SettingsHandler) setAPIKey(w http.ResponseWriter, r *http.Request, provider string, payload map[string]func(*model.UserSettings) any) {
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
	settings, err := h.settings.SetAPIKey(r.Context(), userID, provider, key)
	if err != nil {
		if err.Error() == "user secret encryption is not configured" {
			http.Error(w, "user secret encryption is not configured", http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := map[string]any{"user_id": settings.UserID}
	for k, fn := range payload {
		resp[k] = fn(settings)
	}
	writeJSON(w, resp)
}

func (h *SettingsHandler) deleteAPIKey(w http.ResponseWriter, r *http.Request, provider string, payload map[string]func(*model.UserSettings) any) {
	userID := middleware.GetUserID(r)
	settings, err := h.settings.DeleteAPIKey(r.Context(), userID, provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := map[string]any{"user_id": settings.UserID}
	for k, fn := range payload {
		resp[k] = fn(settings)
	}
	writeJSON(w, resp)
}

func (h *SettingsHandler) SetAnthropicAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "anthropic", map[string]func(*model.UserSettings) any{
		"has_anthropic_api_key":   func(s *model.UserSettings) any { return s.HasAnthropicAPIKey },
		"anthropic_api_key_last4": func(s *model.UserSettings) any { return s.AnthropicAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteAnthropicAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "anthropic", map[string]func(*model.UserSettings) any{
		"has_anthropic_api_key":   func(s *model.UserSettings) any { return s.HasAnthropicAPIKey },
		"anthropic_api_key_last4": func(s *model.UserSettings) any { return s.AnthropicAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetOpenAIAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "openai", map[string]func(*model.UserSettings) any{
		"has_openai_api_key":   func(s *model.UserSettings) any { return s.HasOpenAIAPIKey },
		"openai_api_key_last4": func(s *model.UserSettings) any { return s.OpenAIAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteOpenAIAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "openai", map[string]func(*model.UserSettings) any{
		"has_openai_api_key":   func(s *model.UserSettings) any { return s.HasOpenAIAPIKey },
		"openai_api_key_last4": func(s *model.UserSettings) any { return s.OpenAIAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetGoogleAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "google", map[string]func(*model.UserSettings) any{
		"has_google_api_key":   func(s *model.UserSettings) any { return s.HasGoogleAPIKey },
		"google_api_key_last4": func(s *model.UserSettings) any { return s.GoogleAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteGoogleAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "google", map[string]func(*model.UserSettings) any{
		"has_google_api_key":   func(s *model.UserSettings) any { return s.HasGoogleAPIKey },
		"google_api_key_last4": func(s *model.UserSettings) any { return s.GoogleAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetGroqAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "groq", map[string]func(*model.UserSettings) any{
		"has_groq_api_key":   func(s *model.UserSettings) any { return s.HasGroqAPIKey },
		"groq_api_key_last4": func(s *model.UserSettings) any { return s.GroqAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteGroqAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "groq", map[string]func(*model.UserSettings) any{
		"has_groq_api_key":   func(s *model.UserSettings) any { return s.HasGroqAPIKey },
		"groq_api_key_last4": func(s *model.UserSettings) any { return s.GroqAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetDeepSeekAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "deepseek", map[string]func(*model.UserSettings) any{
		"has_deepseek_api_key":   func(s *model.UserSettings) any { return s.HasDeepSeekAPIKey },
		"deepseek_api_key_last4": func(s *model.UserSettings) any { return s.DeepSeekAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteDeepSeekAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "deepseek", map[string]func(*model.UserSettings) any{
		"has_deepseek_api_key":   func(s *model.UserSettings) any { return s.HasDeepSeekAPIKey },
		"deepseek_api_key_last4": func(s *model.UserSettings) any { return s.DeepSeekAPIKeyLast4 },
	})
}
