package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type SettingsHandler struct {
	settings          *service.SettingsService
	podcastArtwork    *service.PodcastArtworkService
	aivisDictionaries *service.AivisUserDictionaryService
	obsidianRepo      *repository.ObsidianExportRepo
	notificationRepo  *repository.NotificationPriorityRepo
	prefProfileRepo   *repository.PreferenceProfileRepo
	oauth             *service.InoreaderOAuthService
	github            *service.GitHubAppClient
	obsidianExport    *service.ObsidianExportService
	cache             service.JSONCache
}

const settingsCacheTTL = 2 * time.Minute

type navigatorPersonaTaskHints struct {
	CommentRange string `json:"comment_range,omitempty"`
	IntroRange   string `json:"intro_range,omitempty"`
	IntroStyle   string `json:"intro_style,omitempty"`
	Style        string `json:"style,omitempty"`
}

type navigatorPersonaSamplingProfile struct {
	TemperatureHint string `json:"temperature_hint,omitempty"`
	TopPHint        string `json:"top_p_hint,omitempty"`
	VerbosityHint   string `json:"verbosity_hint,omitempty"`
}

type navigatorPersonaDefinition struct {
	Name            string                          `json:"name"`
	Gender          string                          `json:"gender"`
	AgeVibe         string                          `json:"age_vibe"`
	FirstPerson     string                          `json:"first_person"`
	SpeechStyle     string                          `json:"speech_style"`
	Occupation      string                          `json:"occupation"`
	Experience      string                          `json:"experience"`
	Personality     string                          `json:"personality"`
	Values          string                          `json:"values"`
	Interests       string                          `json:"interests"`
	Dislikes        string                          `json:"dislikes"`
	Voice           string                          `json:"voice"`
	SamplingProfile navigatorPersonaSamplingProfile `json:"sampling_profile,omitempty"`
	Briefing        navigatorPersonaTaskHints       `json:"briefing"`
	Item            navigatorPersonaTaskHints       `json:"item"`
}

func NewSettingsHandler(repo *repository.UserSettingsRepo, audioBriefingRepo *repository.AudioBriefingRepo, aivisModelRepo *repository.AivisModelRepo, obsidianRepo *repository.ObsidianExportRepo, notificationRepo *repository.NotificationPriorityRepo, prefProfileRepo *repository.PreferenceProfileRepo, llmUsageRepo *repository.LLMUsageLogRepo, openRouterOverrideRepo *repository.OpenRouterModelOverrideRepo, cipher *service.SecretCipher, github *service.GitHubAppClient, obsidianExport *service.ObsidianExportService, worker *service.WorkerClient, cache service.JSONCache) *SettingsHandler {
	return &SettingsHandler{
		settings:          service.NewSettingsService(repo, audioBriefingRepo, aivisModelRepo, obsidianRepo, llmUsageRepo, openRouterOverrideRepo, cipher, github),
		podcastArtwork:    service.NewPodcastArtworkService(repo, worker),
		aivisDictionaries: service.NewAivisUserDictionaryService(repo, cipher),
		obsidianRepo:      obsidianRepo,
		notificationRepo:  notificationRepo,
		prefProfileRepo:   prefProfileRepo,
		oauth:             service.NewInoreaderOAuthService(repo, cipher),
		github:            github,
		obsidianExport:    obsidianExport,
		cache:             cache,
	}
}

func (h *SettingsHandler) settingsCacheKey(ctx context.Context, userID string) (string, error) {
	version := int64(0)
	if h.cache != nil {
		var err error
		version, err = h.cache.GetVersion(ctx, cacheVersionKeyUserSettings(userID))
		if err != nil {
			return "", err
		}
	}
	return cacheKeySettingsGetVersioned(userID, version), nil
}

func (h *SettingsHandler) bumpUserSettingsVersion(ctx context.Context, userID string) error {
	if h.cache == nil || userID == "" {
		return nil
	}
	_, err := h.cache.BumpVersion(ctx, cacheVersionKeyUserSettings(userID))
	return err
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cacheKey, cacheKeyErr := h.settingsCacheKey(r.Context(), userID)
	if cacheKeyErr != nil {
		log.Printf("settings cache key failed user_id=%s err=%v", userID, cacheKeyErr)
	}
	if h.cache != nil && cacheKeyErr == nil && r.URL.Query().Get("cache_bust") != "1" {
		var cached service.SettingsGetPayload
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			writeJSON(w, &cached)
			return
		} else if err != nil {
			log.Printf("settings cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	payload, err := h.settings.Get(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	if h.notificationRepo != nil {
		if rule, ruleErr := h.notificationRepo.EnsureDefaults(r.Context(), userID); ruleErr == nil {
			payload.NotificationPriority = serviceMapNotificationPriority(rule)
		}
	}
	if h.cache != nil && cacheKeyErr == nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, payload, settingsCacheTTL); err != nil {
			log.Printf("settings cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, payload)
}

func (h *SettingsHandler) GetLLMCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.settings.LLMCatalog(r.Context(), middleware.GetUserID(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, catalog)
}

func (h *SettingsHandler) GetNavigatorPersonas(w http.ResponseWriter, r *http.Request) {
	personaPath, err := resolveNavigatorPersonasPath()
	if err != nil {
		log.Printf("navigator persona resolve failed err=%v", err)
		http.Error(w, "failed to resolve persona definitions", http.StatusInternalServerError)
		return
	}
	body, err := os.ReadFile(personaPath)
	if err != nil {
		log.Printf("navigator persona read failed path=%s err=%v", personaPath, err)
		http.Error(w, "failed to load persona definitions", http.StatusInternalServerError)
		return
	}
	var payload map[string]navigatorPersonaDefinition
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("navigator persona parse failed path=%s err=%v", personaPath, err)
		http.Error(w, "failed to parse persona definitions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, payload)
}

func resolveNavigatorPersonasPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("NAVIGATOR_PERSONAS_PATH")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("LLM_CATALOG_PATH")); v != "" {
		return filepath.Join(filepath.Dir(v), "ai_navigator_personas.json"), nil
	}
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(execPath), "shared", "ai_navigator_personas.json"), nil
}

func (h *SettingsHandler) InoreaderConnect(w http.ResponseWriter, r *http.Request) {
	result, err := h.oauth.BuildConnect(r)
	if err != nil {
		if err.Error() == "inoreader oauth is not configured" {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, "failed to build oauth state", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "inoreader_oauth_state",
		Value:    result.State,
		Path:     "/",
		HttpOnly: true,
		Secure:   result.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   10 * 60,
	})
	http.Redirect(w, r, result.URL, http.StatusFound)
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
	if err := h.oauth.Complete(r.Context(), userID, code, h.oauth.RedirectURIFromRequest(r)); err != nil {
		http.Redirect(w, r, "/settings?inoreader=error&reason="+err.Error(), http.StatusFound)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
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
	settings, err := h.oauth.Clear(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":                 settings.UserID,
		"has_inoreader_oauth":     settings.HasInoreaderOAuth,
		"inoreader_token_expires": settings.InoreaderTokenExpiresAt,
	})
}

func (h *SettingsHandler) ObsidianGitHubConnect(w http.ResponseWriter, r *http.Request) {
	if h.github == nil || strings.TrimSpace(h.github.InstallURL()) == "" {
		http.Error(w, "github app is not configured", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, h.github.InstallURL(), http.StatusFound)
}

func (h *SettingsHandler) ObsidianGitHubCallback(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if h.github == nil || !h.github.Enabled() {
		http.Redirect(w, r, "/settings?obsidian_github=error&reason=disabled", http.StatusFound)
		return
	}
	installationID, err := service.ParseGitHubInstallationID(r.URL.Query().Get("installation_id"))
	if err != nil || installationID <= 0 {
		http.Redirect(w, r, "/settings?obsidian_github=error&reason=invalid_installation", http.StatusFound)
		return
	}
	if _, err := h.settings.UpsertObsidianGitHubInstallation(r.Context(), userID, installationID); err != nil {
		http.Redirect(w, r, "/settings?obsidian_github=error&reason=save_failed", http.StatusFound)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	http.Redirect(w, r, "/settings?obsidian_github=connected", http.StatusFound)
}

func (h *SettingsHandler) UpdateLLMModels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Facts                       *string `json:"facts"`
		FactsFallback               *string `json:"facts_fallback"`
		Summary                     *string `json:"summary"`
		SummaryFallback             *string `json:"summary_fallback"`
		DigestCluster               *string `json:"digest_cluster"`
		Digest                      *string `json:"digest"`
		Ask                         *string `json:"ask"`
		SourceSuggestion            *string `json:"source_suggestion"`
		Embedding                   *string `json:"embedding"`
		FactsCheck                  *string `json:"facts_check"`
		FaithfulnessCheck           *string `json:"faithfulness_check"`
		NavigatorEnabled            bool    `json:"navigator_enabled"`
		AINavigatorBriefEnabled     bool    `json:"ai_navigator_brief_enabled"`
		NavigatorPersonaMode        *string `json:"navigator_persona_mode"`
		NavigatorPersona            *string `json:"navigator_persona"`
		Navigator                   *string `json:"navigator"`
		NavigatorFallback           *string `json:"navigator_fallback"`
		AINavigatorBrief            *string `json:"ai_navigator_brief"`
		AINavigatorBriefFallback    *string `json:"ai_navigator_brief_fallback"`
		AudioBriefingScript         *string `json:"audio_briefing_script"`
		AudioBriefingScriptFallback *string `json:"audio_briefing_script_fallback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	settings, err := h.settings.UpdateLLMModels(r.Context(), userID, service.UpdateLLMModelsInput{
		Facts:                       body.Facts,
		FactsFallback:               body.FactsFallback,
		Summary:                     body.Summary,
		SummaryFallback:             body.SummaryFallback,
		DigestCluster:               body.DigestCluster,
		Digest:                      body.Digest,
		Ask:                         body.Ask,
		SourceSuggestion:            body.SourceSuggestion,
		Embedding:                   body.Embedding,
		FactsCheck:                  body.FactsCheck,
		FaithfulnessCheck:           body.FaithfulnessCheck,
		NavigatorEnabled:            body.NavigatorEnabled,
		AINavigatorBriefEnabled:     body.AINavigatorBriefEnabled,
		NavigatorPersonaMode:        body.NavigatorPersonaMode,
		NavigatorPersona:            body.NavigatorPersona,
		Navigator:                   body.Navigator,
		NavigatorFallback:           body.NavigatorFallback,
		AINavigatorBrief:            body.AINavigatorBrief,
		AINavigatorBriefFallback:    body.AINavigatorBriefFallback,
		AudioBriefingScript:         body.AudioBriefingScript,
		AudioBriefingScriptFallback: body.AudioBriefingScriptFallback,
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "invalid model for ") || strings.HasPrefix(err.Error(), "model missing required capability for ") || err.Error() == "invalid embedding model" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":    settings.UserID,
		"llm_models": service.LLMModelSettingsPayload(settings),
	})
}

func (h *SettingsHandler) UpdateAudioBriefing(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Enabled               bool    `json:"enabled"`
		IntervalHours         int     `json:"interval_hours"`
		ArticlesPerEpisode    int     `json:"articles_per_episode"`
		TargetDurationMinutes int     `json:"target_duration_minutes"`
		DefaultPersonaMode    *string `json:"default_persona_mode"`
		DefaultPersona        *string `json:"default_persona"`
		BGMEnabled            bool    `json:"bgm_enabled"`
		BGMR2Prefix           *string `json:"bgm_r2_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	settings, err := h.settings.UpdateAudioBriefingSettings(r.Context(), userID, service.UpdateAudioBriefingSettingsInput{
		Enabled:               body.Enabled,
		IntervalHours:         body.IntervalHours,
		ArticlesPerEpisode:    body.ArticlesPerEpisode,
		TargetDurationMinutes: body.TargetDurationMinutes,
		DefaultPersonaMode:    body.DefaultPersonaMode,
		DefaultPersona:        body.DefaultPersona,
		BGMEnabled:            body.BGMEnabled,
		BGMR2Prefix:           body.BGMR2Prefix,
	})
	if err != nil {
		switch err.Error() {
		case "invalid interval_hours", "invalid articles_per_episode", "invalid target_duration_minutes", "invalid bgm_r2_prefix":
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":        settings.UserID,
		"audio_briefing": service.AudioBriefingSettingsPayload(settings),
	})
}

func (h *SettingsHandler) UpdatePodcast(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Enabled     bool    `json:"enabled"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Author      *string `json:"author"`
		Language    *string `json:"language"`
		Category    *string `json:"category"`
		Subcategory *string `json:"subcategory"`
		Explicit    bool    `json:"explicit"`
		ArtworkURL  *string `json:"artwork_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	settings, err := h.settings.UpdatePodcastSettings(r.Context(), userID, service.UpdatePodcastSettingsInput{
		Enabled:     body.Enabled,
		Title:       body.Title,
		Description: body.Description,
		Author:      body.Author,
		Language:    body.Language,
		Category:    body.Category,
		Subcategory: body.Subcategory,
		Explicit:    body.Explicit,
		ArtworkURL:  body.ArtworkURL,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidPodcastCategory()) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id": settings.UserID,
		"podcast": service.PodcastSettingsPayload(settings),
	})
}

func (h *SettingsHandler) UploadPodcastArtwork(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if h.podcastArtwork == nil {
		http.Error(w, "podcast artwork unavailable", http.StatusInternalServerError)
		return
	}
	var body struct {
		ContentType   string `json:"content_type"`
		ContentBase64 string `json:"content_base64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	artworkURL, err := h.podcastArtwork.Upload(r.Context(), userID, body.ContentType, body.ContentBase64)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported podcast artwork content_type") || strings.Contains(err.Error(), "AUDIO_BRIEFING_PUBLIC_BASE_URL is not configured") || strings.Contains(err.Error(), "AUDIO_BRIEFING_PUBLIC_BUCKET is not configured") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":     userID,
		"artwork_url": artworkURL,
	})
}

func (h *SettingsHandler) UpdateAudioBriefingPersonaVoices(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Voices []struct {
			Persona                 string  `json:"persona"`
			TTSProvider             string  `json:"tts_provider"`
			VoiceModel              string  `json:"voice_model"`
			VoiceStyle              string  `json:"voice_style"`
			SpeechRate              float64 `json:"speech_rate"`
			EmotionalIntensity      float64 `json:"emotional_intensity"`
			TempoDynamics           float64 `json:"tempo_dynamics"`
			LineBreakSilenceSeconds float64 `json:"line_break_silence_seconds"`
			Pitch                   float64 `json:"pitch"`
			VolumeGain              float64 `json:"volume_gain"`
		} `json:"voices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	inputs := make([]service.UpdateAudioBriefingPersonaVoiceInput, 0, len(body.Voices))
	for _, voice := range body.Voices {
		inputs = append(inputs, service.UpdateAudioBriefingPersonaVoiceInput{
			Persona:                 voice.Persona,
			TTSProvider:             voice.TTSProvider,
			VoiceModel:              voice.VoiceModel,
			VoiceStyle:              voice.VoiceStyle,
			SpeechRate:              voice.SpeechRate,
			EmotionalIntensity:      voice.EmotionalIntensity,
			TempoDynamics:           voice.TempoDynamics,
			LineBreakSilenceSeconds: voice.LineBreakSilenceSeconds,
			Pitch:                   voice.Pitch,
			VolumeGain:              voice.VolumeGain,
		})
	}
	rows, err := h.settings.UpdateAudioBriefingPersonaVoices(r.Context(), userID, inputs)
	if err != nil {
		if strings.HasPrefix(err.Error(), "invalid ") || strings.HasPrefix(err.Error(), "duplicate persona voice:") || strings.HasPrefix(err.Error(), "aivis models are not synced") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":                       userID,
		"audio_briefing_persona_voices": service.AudioBriefingPersonaVoicesPayload(rows),
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
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
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

func (h *SettingsHandler) UpdateObsidianExport(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Enabled          bool    `json:"enabled"`
		GitHubRepoOwner  *string `json:"github_repo_owner"`
		GitHubRepoName   *string `json:"github_repo_name"`
		GitHubRepoBranch *string `json:"github_repo_branch"`
		VaultRootPath    *string `json:"vault_root_path"`
		KeywordLinkMode  *string `json:"keyword_link_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	settings, err := h.settings.UpdateObsidianExport(r.Context(), userID, service.UpdateObsidianExportInput{
		Enabled:          body.Enabled,
		GitHubRepoOwner:  body.GitHubRepoOwner,
		GitHubRepoName:   body.GitHubRepoName,
		GitHubRepoBranch: body.GitHubRepoBranch,
		VaultRootPath:    body.VaultRootPath,
		KeywordLinkMode:  body.KeywordLinkMode,
	})
	if err != nil {
		if err.Error() == "invalid keyword_link_mode" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":         settings.UserID,
		"obsidian_export": serviceMapObsidianExport(settings, h.github),
	})
}

func (h *SettingsHandler) UpdateNotificationPriority(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if h.notificationRepo == nil {
		http.Error(w, "notification priority unavailable", http.StatusInternalServerError)
		return
	}
	var body struct {
		Sensitivity      string  `json:"sensitivity"`
		DailyCap         int     `json:"daily_cap"`
		ThemeWeight      float64 `json:"theme_weight"`
		ImmediateEnabled bool    `json:"immediate_enabled"`
		BriefingEnabled  bool    `json:"briefing_enabled"`
		ReviewEnabled    bool    `json:"review_enabled"`
		GoalMatchEnabled bool    `json:"goal_match_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Sensitivity != "low" && body.Sensitivity != "medium" && body.Sensitivity != "high" {
		http.Error(w, "invalid sensitivity", http.StatusBadRequest)
		return
	}
	if body.DailyCap < 0 || body.DailyCap > 20 {
		http.Error(w, "invalid daily_cap", http.StatusBadRequest)
		return
	}
	if body.ThemeWeight < 0.5 || body.ThemeWeight > 2.0 {
		http.Error(w, "invalid theme_weight", http.StatusBadRequest)
		return
	}
	rule, err := h.notificationRepo.Upsert(r.Context(), userID, body.Sensitivity, body.DailyCap, body.ThemeWeight, body.ImmediateEnabled, body.BriefingEnabled, body.ReviewEnabled, body.GoalMatchEnabled)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":               userID,
		"notification_priority": serviceMapNotificationPriority(rule),
	})
}

func (h *SettingsHandler) RunObsidianExport(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if h.obsidianExport == nil {
		http.Error(w, "obsidian export unavailable", http.StatusInternalServerError)
		return
	}
	if h.obsidianRepo == nil {
		http.Error(w, "obsidian export unavailable", http.StatusInternalServerError)
		return
	}
	cfg, err := h.obsidianRepo.EnsureDefaults(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	res, err := h.obsidianExport.RunUser(r.Context(), *cfg, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func serviceMapObsidianExport(settings *model.ObsidianExportSettings, github *service.GitHubAppClient) map[string]any {
	out := map[string]any{
		"enabled":                settings.Enabled,
		"github_installation_id": settings.GitHubInstallationID,
		"github_repo_owner":      settings.GitHubRepoOwner,
		"github_repo_name":       settings.GitHubRepoName,
		"github_repo_branch":     settings.GitHubRepoBranch,
		"vault_root_path":        settings.VaultRootPath,
		"keyword_link_mode":      settings.KeywordLinkMode,
		"last_run_at":            settings.LastRunAt,
		"last_success_at":        settings.LastSuccessAt,
	}
	if github != nil {
		out["github_app_enabled"] = github.Enabled()
		out["github_app_install_url"] = github.InstallURL()
	}
	return out
}

func serviceMapNotificationPriority(rule *model.NotificationPriorityRule) map[string]any {
	if rule == nil {
		return map[string]any{
			"sensitivity":        "medium",
			"daily_cap":          3,
			"theme_weight":       1.0,
			"immediate_enabled":  true,
			"briefing_enabled":   true,
			"review_enabled":     true,
			"goal_match_enabled": true,
		}
	}
	return map[string]any{
		"id":                 rule.ID,
		"sensitivity":        rule.Sensitivity,
		"daily_cap":          rule.DailyCap,
		"theme_weight":       rule.ThemeWeight,
		"immediate_enabled":  rule.ImmediateEnabled,
		"briefing_enabled":   rule.BriefingEnabled,
		"review_enabled":     rule.ReviewEnabled,
		"goal_match_enabled": rule.GoalMatchEnabled,
	}
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
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
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
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
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
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
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

func (h *SettingsHandler) SetAlibabaAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "alibaba", map[string]func(*model.UserSettings) any{
		"has_alibaba_api_key":   func(s *model.UserSettings) any { return s.HasAlibabaAPIKey },
		"alibaba_api_key_last4": func(s *model.UserSettings) any { return s.AlibabaAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteAlibabaAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "alibaba", map[string]func(*model.UserSettings) any{
		"has_alibaba_api_key":   func(s *model.UserSettings) any { return s.HasAlibabaAPIKey },
		"alibaba_api_key_last4": func(s *model.UserSettings) any { return s.AlibabaAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetMistralAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "mistral", map[string]func(*model.UserSettings) any{
		"has_mistral_api_key":   func(s *model.UserSettings) any { return s.HasMistralAPIKey },
		"mistral_api_key_last4": func(s *model.UserSettings) any { return s.MistralAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteMistralAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "mistral", map[string]func(*model.UserSettings) any{
		"has_mistral_api_key":   func(s *model.UserSettings) any { return s.HasMistralAPIKey },
		"mistral_api_key_last4": func(s *model.UserSettings) any { return s.MistralAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetMoonshotAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "moonshot", map[string]func(*model.UserSettings) any{
		"has_moonshot_api_key":   func(s *model.UserSettings) any { return s.HasMoonshotAPIKey },
		"moonshot_api_key_last4": func(s *model.UserSettings) any { return s.MoonshotAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteMoonshotAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "moonshot", map[string]func(*model.UserSettings) any{
		"has_moonshot_api_key":   func(s *model.UserSettings) any { return s.HasMoonshotAPIKey },
		"moonshot_api_key_last4": func(s *model.UserSettings) any { return s.MoonshotAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetXAIAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "xai", map[string]func(*model.UserSettings) any{
		"has_xai_api_key":   func(s *model.UserSettings) any { return s.HasXAIAPIKey },
		"xai_api_key_last4": func(s *model.UserSettings) any { return s.XAIAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteXAIAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "xai", map[string]func(*model.UserSettings) any{
		"has_xai_api_key":   func(s *model.UserSettings) any { return s.HasXAIAPIKey },
		"xai_api_key_last4": func(s *model.UserSettings) any { return s.XAIAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetZAIAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "zai", map[string]func(*model.UserSettings) any{
		"has_zai_api_key":   func(s *model.UserSettings) any { return s.HasZAIAPIKey },
		"zai_api_key_last4": func(s *model.UserSettings) any { return s.ZAIAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteZAIAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "zai", map[string]func(*model.UserSettings) any{
		"has_zai_api_key":   func(s *model.UserSettings) any { return s.HasZAIAPIKey },
		"zai_api_key_last4": func(s *model.UserSettings) any { return s.ZAIAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetFireworksAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "fireworks", map[string]func(*model.UserSettings) any{
		"has_fireworks_api_key":   func(s *model.UserSettings) any { return s.HasFireworksAPIKey },
		"fireworks_api_key_last4": func(s *model.UserSettings) any { return s.FireworksAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteFireworksAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "fireworks", map[string]func(*model.UserSettings) any{
		"has_fireworks_api_key":   func(s *model.UserSettings) any { return s.HasFireworksAPIKey },
		"fireworks_api_key_last4": func(s *model.UserSettings) any { return s.FireworksAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetPoeAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "poe", map[string]func(*model.UserSettings) any{
		"has_poe_api_key":   func(s *model.UserSettings) any { return s.HasPoeAPIKey },
		"poe_api_key_last4": func(s *model.UserSettings) any { return s.PoeAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeletePoeAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "poe", map[string]func(*model.UserSettings) any{
		"has_poe_api_key":   func(s *model.UserSettings) any { return s.HasPoeAPIKey },
		"poe_api_key_last4": func(s *model.UserSettings) any { return s.PoeAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetOpenRouterAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "openrouter", map[string]func(*model.UserSettings) any{
		"has_openrouter_api_key":   func(s *model.UserSettings) any { return s.HasOpenRouterAPIKey },
		"openrouter_api_key_last4": func(s *model.UserSettings) any { return s.OpenRouterAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteOpenRouterAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "openrouter", map[string]func(*model.UserSettings) any{
		"has_openrouter_api_key":   func(s *model.UserSettings) any { return s.HasOpenRouterAPIKey },
		"openrouter_api_key_last4": func(s *model.UserSettings) any { return s.OpenRouterAPIKeyLast4 },
	})
}

func (h *SettingsHandler) SetAivisAPIKey(w http.ResponseWriter, r *http.Request) {
	h.setAPIKey(w, r, "aivis", map[string]func(*model.UserSettings) any{
		"has_aivis_api_key":   func(s *model.UserSettings) any { return s.HasAivisAPIKey },
		"aivis_api_key_last4": func(s *model.UserSettings) any { return s.AivisAPIKeyLast4 },
	})
}

func (h *SettingsHandler) DeleteAivisAPIKey(w http.ResponseWriter, r *http.Request) {
	h.deleteAPIKey(w, r, "aivis", map[string]func(*model.UserSettings) any{
		"has_aivis_api_key":   func(s *model.UserSettings) any { return s.HasAivisAPIKey },
		"aivis_api_key_last4": func(s *model.UserSettings) any { return s.AivisAPIKeyLast4 },
	})
}

func (h *SettingsHandler) GetAivisUserDictionaries(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	items, err := h.aivisDictionaries.List(r.Context(), userID)
	if err != nil {
		switch err.Error() {
		case "aivis api key is not configured":
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case "user secret encryption is not configured":
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		default:
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	writeJSON(w, map[string]any{"user_dictionaries": items})
}

func (h *SettingsHandler) SetAivisUserDictionary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		AivisUserDictionaryUUID string `json:"aivis_user_dictionary_uuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	settings, err := h.settings.SetAivisUserDictionaryUUID(r.Context(), userID, body.AivisUserDictionaryUUID)
	if err != nil {
		if err.Error() == "aivis_user_dictionary_uuid is required" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":                    settings.UserID,
		"aivis_user_dictionary_uuid": settings.AivisUserDictionaryUUID,
	})
}

func (h *SettingsHandler) DeleteAivisUserDictionary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	settings, err := h.settings.ClearAivisUserDictionaryUUID(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.bumpUserSettingsVersion(r.Context(), userID); err != nil {
		log.Printf("settings version bump failed user_id=%s err=%v", userID, err)
	}
	writeJSON(w, map[string]any{
		"user_id":                    settings.UserID,
		"aivis_user_dictionary_uuid": settings.AivisUserDictionaryUUID,
	})
}
