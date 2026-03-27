package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

type SettingsService struct {
	repo                   *repository.UserSettingsRepo
	audioBriefingRepo      *repository.AudioBriefingRepo
	aivisModelRepo         *repository.AivisModelRepo
	obsidianRepo           *repository.ObsidianExportRepo
	llmUsageRepo           *repository.LLMUsageLogRepo
	openRouterOverrideRepo *repository.OpenRouterModelOverrideRepo
	cipher                 *SecretCipher
	githubApp              *GitHubAppClient
}

type SettingsGetPayload struct {
	UserID                  string           `json:"user_id"`
	HasAnthropicAPIKey      bool             `json:"has_anthropic_api_key"`
	AnthropicAPIKeyLast4    *string          `json:"anthropic_api_key_last4,omitempty"`
	HasOpenAIAPIKey         bool             `json:"has_openai_api_key"`
	OpenAIAPIKeyLast4       *string          `json:"openai_api_key_last4,omitempty"`
	HasGoogleAPIKey         bool             `json:"has_google_api_key"`
	GoogleAPIKeyLast4       *string          `json:"google_api_key_last4,omitempty"`
	HasGroqAPIKey           bool             `json:"has_groq_api_key"`
	GroqAPIKeyLast4         *string          `json:"groq_api_key_last4,omitempty"`
	HasDeepSeekAPIKey       bool             `json:"has_deepseek_api_key"`
	DeepSeekAPIKeyLast4     *string          `json:"deepseek_api_key_last4,omitempty"`
	HasAlibabaAPIKey        bool             `json:"has_alibaba_api_key"`
	AlibabaAPIKeyLast4      *string          `json:"alibaba_api_key_last4,omitempty"`
	HasMistralAPIKey        bool             `json:"has_mistral_api_key"`
	MistralAPIKeyLast4      *string          `json:"mistral_api_key_last4,omitempty"`
	HasXAIAPIKey            bool             `json:"has_xai_api_key"`
	XAIAPIKeyLast4          *string          `json:"xai_api_key_last4,omitempty"`
	HasZAIAPIKey            bool             `json:"has_zai_api_key"`
	ZAIAPIKeyLast4          *string          `json:"zai_api_key_last4,omitempty"`
	HasFireworksAPIKey      bool             `json:"has_fireworks_api_key"`
	FireworksAPIKeyLast4    *string          `json:"fireworks_api_key_last4,omitempty"`
	HasPoeAPIKey            bool             `json:"has_poe_api_key"`
	PoeAPIKeyLast4          *string          `json:"poe_api_key_last4,omitempty"`
	HasOpenRouterAPIKey     bool             `json:"has_openrouter_api_key"`
	OpenRouterAPIKeyLast4   *string          `json:"openrouter_api_key_last4,omitempty"`
	HasAivisAPIKey          bool             `json:"has_aivis_api_key"`
	AivisAPIKeyLast4        *string          `json:"aivis_api_key_last4,omitempty"`
	AivisUserDictionaryUUID *string          `json:"aivis_user_dictionary_uuid,omitempty"`
	Podcast                 map[string]any   `json:"podcast"`
	HasInoreaderOAuth       bool             `json:"has_inoreader_oauth"`
	InoreaderTokenExpiresAt *time.Time       `json:"inoreader_token_expires_at,omitempty"`
	MonthlyBudgetUSD        *float64         `json:"monthly_budget_usd,omitempty"`
	BudgetAlertEnabled      bool             `json:"budget_alert_enabled"`
	BudgetAlertThresholdPct int              `json:"budget_alert_threshold_pct"`
	DigestEmailEnabled      bool             `json:"digest_email_enabled"`
	ReadingPlan             map[string]any   `json:"reading_plan"`
	LLMModels               map[string]any   `json:"llm_models"`
	AudioBriefing           map[string]any   `json:"audio_briefing"`
	AudioBriefingVoices     []map[string]any `json:"audio_briefing_persona_voices"`
	CurrentMonth            map[string]any   `json:"current_month"`
	ObsidianExport          map[string]any   `json:"obsidian_export"`
	NotificationPriority    map[string]any   `json:"notification_priority"`
}

type UpdateLLMModelsInput struct {
	Facts                       *string
	FactsFallback               *string
	Summary                     *string
	SummaryFallback             *string
	DigestCluster               *string
	Digest                      *string
	Ask                         *string
	SourceSuggestion            *string
	Embedding                   *string
	FactsCheck                  *string
	FaithfulnessCheck           *string
	NavigatorEnabled            bool
	AINavigatorBriefEnabled     bool
	NavigatorPersonaMode        *string
	NavigatorPersona            *string
	Navigator                   *string
	NavigatorFallback           *string
	AudioBriefingScript         *string
	AudioBriefingScriptFallback *string
}

type UpdateAudioBriefingSettingsInput struct {
	Enabled               bool
	IntervalHours         int
	ArticlesPerEpisode    int
	TargetDurationMinutes int
	DefaultPersonaMode    *string
	DefaultPersona        *string
}

type UpdatePodcastSettingsInput struct {
	Enabled     bool
	Title       *string
	Description *string
	Author      *string
	Language    *string
	Category    *string
	Subcategory *string
	Explicit    bool
	ArtworkURL  *string
}

var errInvalidPodcastCategory = errors.New("invalid podcast category")

func ErrInvalidPodcastCategory() error {
	return errInvalidPodcastCategory
}

type UpdateAudioBriefingPersonaVoiceInput struct {
	Persona                 string
	TTSProvider             string
	VoiceModel              string
	VoiceStyle              string
	SpeechRate              float64
	EmotionalIntensity      float64
	TempoDynamics           float64
	LineBreakSilenceSeconds float64
	Pitch                   float64
	VolumeGain              float64
}

var modelSettingPurposes = map[string]string{
	"facts":                          "facts",
	"facts_fallback":                 "facts",
	"summary":                        "summary",
	"summary_fallback":               "summary",
	"digest_cluster":                 "digest_cluster_draft",
	"digest":                         "digest",
	"ask":                            "ask",
	"source_suggestion":              "source_suggestion",
	"facts_check":                    "facts",
	"faithfulness_check":             "summary",
	"navigator":                      "summary",
	"navigator_fallback":             "summary",
	"audio_briefing_script":          "summary",
	"audio_briefing_script_fallback": "summary",
}

var modelSettingRequiredCapabilities = map[string][]string{
	"facts":                          {"structured_output"},
	"facts_fallback":                 {"structured_output"},
	"summary":                        {"structured_output"},
	"summary_fallback":               {"structured_output"},
	"digest_cluster":                 {"structured_output"},
	"digest":                         {"structured_output"},
	"ask":                            {"structured_output"},
	"source_suggestion":              {"structured_output"},
	"facts_check":                    {"structured_output"},
	"faithfulness_check":             {"structured_output"},
	"navigator":                      {"structured_output"},
	"navigator_fallback":             {"structured_output"},
	"audio_briefing_script":          {"structured_output"},
	"audio_briefing_script_fallback": {"structured_output"},
}

func NewSettingsService(repo *repository.UserSettingsRepo, audioBriefingRepo *repository.AudioBriefingRepo, aivisModelRepo *repository.AivisModelRepo, obsidianRepo *repository.ObsidianExportRepo, llmUsageRepo *repository.LLMUsageLogRepo, openRouterOverrideRepo *repository.OpenRouterModelOverrideRepo, cipher *SecretCipher, githubApp *GitHubAppClient) *SettingsService {
	return &SettingsService{
		repo:                   repo,
		audioBriefingRepo:      audioBriefingRepo,
		aivisModelRepo:         aivisModelRepo,
		obsidianRepo:           obsidianRepo,
		llmUsageRepo:           llmUsageRepo,
		openRouterOverrideRepo: openRouterOverrideRepo,
		cipher:                 cipher,
		githubApp:              githubApp,
	}
}

func obsidianExportPayload(settings *model.ObsidianExportSettings, githubApp *GitHubAppClient) map[string]any {
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
	if githubApp != nil {
		out["github_app_enabled"] = githubApp.Enabled()
		out["github_app_install_url"] = githubApp.InstallURL()
	}
	return out
}

func LLMModelSettingsPayload(settings *model.UserSettings) map[string]any {
	return map[string]any{
		"facts":                          settings.FactsModel,
		"facts_fallback":                 settings.FactsFallbackModel,
		"summary":                        settings.SummaryModel,
		"summary_fallback":               settings.SummaryFallbackModel,
		"digest_cluster":                 settings.DigestClusterModel,
		"digest":                         settings.DigestModel,
		"ask":                            settings.AskModel,
		"source_suggestion":              settings.SourceSuggestionModel,
		"embedding":                      settings.EmbeddingModel,
		"facts_check":                    settings.FactsCheckModel,
		"faithfulness_check":             settings.FaithfulnessCheckModel,
		"navigator_enabled":              settings.NavigatorEnabled,
		"ai_navigator_brief_enabled":     settings.AINavigatorBriefEnabled,
		"navigator_persona_mode":         NormalizePersonaMode(&settings.NavigatorPersonaMode),
		"navigator_persona":              settings.NavigatorPersona,
		"navigator":                      settings.NavigatorModel,
		"navigator_fallback":             settings.NavigatorFallbackModel,
		"audio_briefing_script":          settings.AudioBriefingScriptModel,
		"audio_briefing_script_fallback": settings.AudioBriefingScriptFallbackModel,
	}
}

func readingPlanPayload(settings *model.UserSettings) map[string]any {
	return map[string]any{
		"window":           settings.ReadingPlanWindow,
		"size":             settings.ReadingPlanSize,
		"diversify_topics": settings.ReadingPlanDiversifyTopics,
		"exclude_read":     settings.ReadingPlanExcludeRead,
	}
}

func AudioBriefingSettingsPayload(settings *model.AudioBriefingSettings) map[string]any {
	if settings == nil {
		return map[string]any{
			"enabled":                 false,
			"interval_hours":          6,
			"articles_per_episode":    5,
			"target_duration_minutes": 20,
			"default_persona_mode":    PersonaModeFixed,
			"default_persona":         "editor",
		}
	}
	return map[string]any{
		"enabled":                 settings.Enabled,
		"interval_hours":          settings.IntervalHours,
		"articles_per_episode":    settings.ArticlesPerEpisode,
		"target_duration_minutes": settings.TargetDurationMinutes,
		"default_persona_mode":    NormalizePersonaMode(&settings.DefaultPersonaMode),
		"default_persona":         settings.DefaultPersona,
	}
}

func AudioBriefingPersonaVoicesPayload(rows []model.AudioBriefingPersonaVoice) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"persona":                    row.Persona,
			"tts_provider":               row.TTSProvider,
			"voice_model":                row.VoiceModel,
			"voice_style":                row.VoiceStyle,
			"speech_rate":                row.SpeechRate,
			"emotional_intensity":        row.EmotionalIntensity,
			"tempo_dynamics":             row.TempoDynamics,
			"line_break_silence_seconds": row.LineBreakSilenceSeconds,
			"pitch":                      row.Pitch,
			"volume_gain":                row.VolumeGain,
		})
	}
	return out
}

func podcastFeedBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("PODCAST_FEED_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("APP_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/") + "/podcasts"
	}
	return ""
}

func podcastDefaultArtworkURL() *string {
	if v := strings.TrimSpace(os.Getenv("PODCAST_DEFAULT_ARTWORK_URL")); v != "" {
		return &v
	}
	return nil
}

func podcastRSSURL(slug *string) *string {
	if slug == nil || strings.TrimSpace(*slug) == "" {
		return nil
	}
	base := podcastFeedBaseURL()
	if base == "" {
		return nil
	}
	v := base + "/" + strings.TrimSpace(*slug) + "/feed.xml"
	return &v
}

func PodcastSettingsPayload(settings *model.UserSettings) map[string]any {
	var artworkURL *string
	if settings != nil {
		artworkURL = settings.PodcastArtworkURL
	}
	if artworkURL == nil {
		artworkURL = podcastDefaultArtworkURL()
	}
	if settings == nil {
		return map[string]any{
			"enabled":              false,
			"feed_slug":            nil,
			"rss_url":              nil,
			"title":                nil,
			"description":          nil,
			"author":               nil,
			"language":             "ja",
			"category":             nil,
			"subcategory":          nil,
			"available_categories": PodcastCategoryDefinitions(),
			"explicit":             false,
			"artwork_url":          artworkURL,
		}
	}
	return map[string]any{
		"enabled":              settings.PodcastEnabled,
		"feed_slug":            settings.PodcastFeedSlug,
		"rss_url":              podcastRSSURL(settings.PodcastFeedSlug),
		"title":                settings.PodcastTitle,
		"description":          settings.PodcastDescription,
		"author":               settings.PodcastAuthor,
		"language":             settings.PodcastLanguage,
		"category":             settings.PodcastCategory,
		"subcategory":          settings.PodcastSubcategory,
		"available_categories": PodcastCategoryDefinitions(),
		"explicit":             settings.PodcastExplicit,
		"artwork_url":          artworkURL,
	}
}

func (s *SettingsService) Get(ctx context.Context, userID string) (*SettingsGetPayload, error) {
	settings, err := s.repo.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	var audioBriefingSettings *model.AudioBriefingSettings
	var audioBriefingVoices []model.AudioBriefingPersonaVoice
	if s.audioBriefingRepo != nil {
		audioBriefingSettings, err = s.audioBriefingRepo.EnsureSettingsDefaults(ctx, userID)
		if err != nil {
			return nil, err
		}
		audioBriefingVoices, err = s.audioBriefingRepo.ListPersonaVoicesByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	obsidianSettings, err := s.obsidianRepo.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	nowJST := timeutil.NowJST()
	monthStart := time.Date(nowJST.Year(), nowJST.Month(), 1, 0, 0, 0, 0, timeutil.JST)
	nextMonth := monthStart.AddDate(0, 1, 0)
	usedCostUSD, err := s.llmUsageRepo.SumEstimatedCostByUserBetween(ctx, userID, monthStart, nextMonth)
	if err != nil {
		return nil, err
	}
	var remainingBudgetUSD *float64
	var remainingPct *float64
	if settings.MonthlyBudgetUSD != nil && *settings.MonthlyBudgetUSD > 0 {
		v := *settings.MonthlyBudgetUSD - usedCostUSD
		remainingBudgetUSD = &v
		p := (v / *settings.MonthlyBudgetUSD) * 100
		remainingPct = &p
	}
	return &SettingsGetPayload{
		UserID:                  settings.UserID,
		HasAnthropicAPIKey:      settings.HasAnthropicAPIKey,
		AnthropicAPIKeyLast4:    settings.AnthropicAPIKeyLast4,
		HasOpenAIAPIKey:         settings.HasOpenAIAPIKey,
		OpenAIAPIKeyLast4:       settings.OpenAIAPIKeyLast4,
		HasGoogleAPIKey:         settings.HasGoogleAPIKey,
		GoogleAPIKeyLast4:       settings.GoogleAPIKeyLast4,
		HasGroqAPIKey:           settings.HasGroqAPIKey,
		GroqAPIKeyLast4:         settings.GroqAPIKeyLast4,
		HasDeepSeekAPIKey:       settings.HasDeepSeekAPIKey,
		DeepSeekAPIKeyLast4:     settings.DeepSeekAPIKeyLast4,
		HasAlibabaAPIKey:        settings.HasAlibabaAPIKey,
		AlibabaAPIKeyLast4:      settings.AlibabaAPIKeyLast4,
		HasMistralAPIKey:        settings.HasMistralAPIKey,
		MistralAPIKeyLast4:      settings.MistralAPIKeyLast4,
		HasXAIAPIKey:            settings.HasXAIAPIKey,
		XAIAPIKeyLast4:          settings.XAIAPIKeyLast4,
		HasZAIAPIKey:            settings.HasZAIAPIKey,
		ZAIAPIKeyLast4:          settings.ZAIAPIKeyLast4,
		HasFireworksAPIKey:      settings.HasFireworksAPIKey,
		FireworksAPIKeyLast4:    settings.FireworksAPIKeyLast4,
		HasPoeAPIKey:            settings.HasPoeAPIKey,
		PoeAPIKeyLast4:          settings.PoeAPIKeyLast4,
		HasOpenRouterAPIKey:     settings.HasOpenRouterAPIKey,
		OpenRouterAPIKeyLast4:   settings.OpenRouterAPIKeyLast4,
		HasAivisAPIKey:          settings.HasAivisAPIKey,
		AivisAPIKeyLast4:        settings.AivisAPIKeyLast4,
		AivisUserDictionaryUUID: settings.AivisUserDictionaryUUID,
		Podcast:                 PodcastSettingsPayload(settings),
		HasInoreaderOAuth:       settings.HasInoreaderOAuth,
		InoreaderTokenExpiresAt: settings.InoreaderTokenExpiresAt,
		MonthlyBudgetUSD:        settings.MonthlyBudgetUSD,
		BudgetAlertEnabled:      settings.BudgetAlertEnabled,
		BudgetAlertThresholdPct: settings.BudgetAlertThresholdPct,
		DigestEmailEnabled:      settings.DigestEmailEnabled,
		ReadingPlan:             readingPlanPayload(settings),
		LLMModels:               LLMModelSettingsPayload(settings),
		AudioBriefing:           AudioBriefingSettingsPayload(audioBriefingSettings),
		AudioBriefingVoices:     AudioBriefingPersonaVoicesPayload(audioBriefingVoices),
		ObsidianExport:          obsidianExportPayload(obsidianSettings, s.githubApp),
		CurrentMonth: map[string]any{
			"month_jst":            monthStart.Format("2006-01"),
			"period_start_jst":     monthStart.Format(time.RFC3339),
			"period_end_jst":       nextMonth.Format(time.RFC3339),
			"estimated_cost_usd":   usedCostUSD,
			"remaining_budget_usd": remainingBudgetUSD,
			"remaining_budget_pct": remainingPct,
		},
	}, nil
}

func (s *SettingsService) SetAivisUserDictionaryUUID(ctx context.Context, userID, uuid string) (*model.UserSettings, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return nil, fmt.Errorf("aivis_user_dictionary_uuid is required")
	}
	return s.repo.SetAivisUserDictionaryUUID(ctx, userID, uuid)
}

func (s *SettingsService) ClearAivisUserDictionaryUUID(ctx context.Context, userID string) (*model.UserSettings, error) {
	return s.repo.ClearAivisUserDictionaryUUID(ctx, userID)
}

type UpdateObsidianExportInput struct {
	Enabled          bool
	GitHubRepoOwner  *string
	GitHubRepoName   *string
	GitHubRepoBranch *string
	VaultRootPath    *string
	KeywordLinkMode  *string
}

func normalizeOptionalModel(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func normalizeNavigatorPersona(v *string) string {
	if v == nil {
		return "editor"
	}
	return NormalizePersonaValue(*v)
}

func normalizeAudioBriefingDefaultPersona(v *string) string {
	return normalizeNavigatorPersona(v)
}

func validateAudioBriefingPersonaVoiceInputs(rows []UpdateAudioBriefingPersonaVoiceInput) ([]model.AudioBriefingPersonaVoice, error) {
	out := make([]model.AudioBriefingPersonaVoice, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		persona := normalizeAudioBriefingDefaultPersona(&row.Persona)
		if _, ok := seen[persona]; ok {
			return nil, fmt.Errorf("duplicate persona voice: %s", persona)
		}
		seen[persona] = struct{}{}
		provider := strings.TrimSpace(row.TTSProvider)
		if provider == "" {
			return nil, fmt.Errorf("invalid tts_provider for %s", persona)
		}
		voiceModel := strings.TrimSpace(row.VoiceModel)
		voiceStyle := strings.TrimSpace(row.VoiceStyle)
		// Allow incomplete persona rows to remain unset in the UI without persisting
		// placeholder records. Only partially configured rows are rejected.
		if voiceModel == "" && voiceStyle == "" {
			continue
		}
		if voiceModel == "" {
			return nil, fmt.Errorf("invalid voice_model for %s", persona)
		}
		if voiceStyle == "" {
			return nil, fmt.Errorf("invalid voice_style for %s", persona)
		}
		if row.SpeechRate < 0.5 || row.SpeechRate > 2.0 {
			return nil, fmt.Errorf("invalid speech_rate for %s", persona)
		}
		if row.EmotionalIntensity < 0 || row.EmotionalIntensity > 2.0 {
			return nil, fmt.Errorf("invalid emotional_intensity for %s", persona)
		}
		if row.TempoDynamics < 0 || row.TempoDynamics > 2.0 {
			return nil, fmt.Errorf("invalid tempo_dynamics for %s", persona)
		}
		if row.LineBreakSilenceSeconds < 0 || row.LineBreakSilenceSeconds > 5.0 {
			return nil, fmt.Errorf("invalid line_break_silence_seconds for %s", persona)
		}
		if row.Pitch < -12 || row.Pitch > 12 {
			return nil, fmt.Errorf("invalid pitch for %s", persona)
		}
		if row.VolumeGain < -24 || row.VolumeGain > 24 {
			return nil, fmt.Errorf("invalid volume_gain for %s", persona)
		}
		out = append(out, model.AudioBriefingPersonaVoice{
			Persona:                 persona,
			TTSProvider:             provider,
			VoiceModel:              voiceModel,
			VoiceStyle:              voiceStyle,
			SpeechRate:              row.SpeechRate,
			EmotionalIntensity:      row.EmotionalIntensity,
			TempoDynamics:           row.TempoDynamics,
			LineBreakSilenceSeconds: row.LineBreakSilenceSeconds,
			Pitch:                   row.Pitch,
			VolumeGain:              row.VolumeGain,
		})
	}
	return out, nil
}

type aivisVoiceSelection struct {
	SpeakerUUID string `json:"speaker_uuid"`
	StyleID     int    `json:"style_id"`
}

type aivisSpeakerSnapshot struct {
	AivmSpeakerUUID string `json:"aivm_speaker_uuid"`
	Styles          []struct {
		LocalID int `json:"local_id"`
	} `json:"styles"`
}

func parseAivisVoiceStyle(raw string) (*aivisVoiceSelection, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("voice style is empty")
	}
	if strings.HasPrefix(raw, "{") {
		var parsed aivisVoiceSelection
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return nil, fmt.Errorf("voice style json is invalid")
		}
		if strings.TrimSpace(parsed.SpeakerUUID) == "" {
			return nil, fmt.Errorf("speaker uuid is missing")
		}
		return &parsed, nil
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("voice style must be speaker_uuid:style_id")
	}
	styleID, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("voice style style_id is invalid")
	}
	speakerUUID := strings.TrimSpace(parts[0])
	if speakerUUID == "" {
		return nil, fmt.Errorf("speaker uuid is missing")
	}
	return &aivisVoiceSelection{SpeakerUUID: speakerUUID, StyleID: styleID}, nil
}

func validateAivisVoiceSelectionAgainstSnapshots(snapshots []repository.AivisModelSnapshot, voiceModel, voiceStyle string) error {
	modelID := strings.TrimSpace(voiceModel)
	if modelID == "" {
		return fmt.Errorf("voice model is empty")
	}
	styleSelection, err := parseAivisVoiceStyle(voiceStyle)
	if err != nil {
		return err
	}
	var target *repository.AivisModelSnapshot
	for i := range snapshots {
		if snapshots[i].AivmModelUUID == modelID {
			target = &snapshots[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("aivis model is not synced")
	}
	var speakers []aivisSpeakerSnapshot
	if err := json.Unmarshal(target.SpeakersJSON, &speakers); err != nil {
		return fmt.Errorf("aivis speaker catalog is invalid")
	}
	for _, speaker := range speakers {
		if strings.TrimSpace(speaker.AivmSpeakerUUID) != styleSelection.SpeakerUUID {
			continue
		}
		for _, style := range speaker.Styles {
			if style.LocalID == styleSelection.StyleID {
				return nil
			}
		}
	}
	return fmt.Errorf("aivis speaker/style is not synced")
}

func validateCatalogModelForPurpose(catalog *LLMCatalog, model *string, purpose string) error {
	if model == nil {
		return nil
	}
	if !CatalogModelSupportsPurposeInCatalog(catalog, *model, purpose) {
		return fmt.Errorf("invalid model for %s", purpose)
	}
	return nil
}

func validateCatalogModelCapabilities(catalog *LLMCatalog, model *string, settingKey string) error {
	if model == nil {
		return nil
	}
	for _, capability := range modelSettingRequiredCapabilities[settingKey] {
		if !CatalogModelSupportsCapabilityInCatalog(catalog, *model, capability) {
			return fmt.Errorf("model missing required capability for %s", settingKey)
		}
	}
	return nil
}

func (s *SettingsService) LLMCatalog(ctx context.Context, userID string) (*LLMCatalog, error) {
	catalog := LLMCatalogData()
	if s.openRouterOverrideRepo == nil || strings.TrimSpace(userID) == "" {
		return catalog, nil
	}
	overrides, err := s.openRouterOverrideRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return ApplyUserOpenRouterOverridesToCatalog(catalog, overrides), nil
}

func (s *SettingsService) UpdateLLMModels(ctx context.Context, userID string, in UpdateLLMModelsInput) (*model.UserSettings, error) {
	catalog, err := s.LLMCatalog(ctx, userID)
	if err != nil {
		return nil, err
	}
	normalized := map[string]*string{
		"facts":                          normalizeOptionalModel(in.Facts),
		"facts_fallback":                 normalizeOptionalModel(in.FactsFallback),
		"summary":                        normalizeOptionalModel(in.Summary),
		"summary_fallback":               normalizeOptionalModel(in.SummaryFallback),
		"digest_cluster":                 normalizeOptionalModel(in.DigestCluster),
		"digest":                         normalizeOptionalModel(in.Digest),
		"ask":                            normalizeOptionalModel(in.Ask),
		"source_suggestion":              normalizeOptionalModel(in.SourceSuggestion),
		"embedding":                      normalizeOptionalModel(in.Embedding),
		"facts_check":                    normalizeOptionalModel(in.FactsCheck),
		"faithfulness_check":             normalizeOptionalModel(in.FaithfulnessCheck),
		"navigator":                      normalizeOptionalModel(in.Navigator),
		"navigator_fallback":             normalizeOptionalModel(in.NavigatorFallback),
		"audio_briefing_script":          normalizeOptionalModel(in.AudioBriefingScript),
		"audio_briefing_script_fallback": normalizeOptionalModel(in.AudioBriefingScriptFallback),
	}
	for settingKey, purpose := range modelSettingPurposes {
		if err := validateCatalogModelForPurpose(catalog, normalized[settingKey], purpose); err != nil {
			return nil, err
		}
		if err := validateCatalogModelCapabilities(catalog, normalized[settingKey], settingKey); err != nil {
			return nil, err
		}
	}
	embeddingModel := normalized["embedding"]
	if embeddingModel != nil && !CatalogIsEmbeddingModelInCatalog(catalog, *embeddingModel) {
		return nil, fmt.Errorf("invalid embedding model")
	}
	return s.repo.UpsertLLMModelConfig(
		ctx,
		userID,
		normalized["facts"],
		normalized["facts_fallback"],
		normalized["summary"],
		normalized["summary_fallback"],
		normalized["digest_cluster"],
		normalized["digest"],
		normalized["ask"],
		normalized["source_suggestion"],
		embeddingModel,
		normalized["facts_check"],
		normalized["faithfulness_check"],
		in.NavigatorEnabled,
		in.AINavigatorBriefEnabled,
		NormalizePersonaMode(in.NavigatorPersonaMode),
		normalizeNavigatorPersona(in.NavigatorPersona),
		normalized["navigator"],
		normalized["navigator_fallback"],
		normalized["audio_briefing_script"],
		normalized["audio_briefing_script_fallback"],
	)
}

func (s *SettingsService) UpdateReadingPlan(ctx context.Context, userID, window string, size int, diversifyTopics, excludeRead bool) (*model.UserSettings, error) {
	return s.repo.UpsertReadingPlanConfig(ctx, userID, window, size, diversifyTopics, excludeRead)
}

func (s *SettingsService) UpdateBudget(ctx context.Context, userID string, monthlyBudgetUSD *float64, enabled bool, thresholdPct int, digestEmailEnabled bool) (*model.UserSettings, error) {
	var budget *float64
	if monthlyBudgetUSD != nil && *monthlyBudgetUSD > 0 {
		budget = monthlyBudgetUSD
	}
	return s.repo.UpsertBudgetConfig(ctx, userID, budget, enabled, thresholdPct, digestEmailEnabled)
}

func (s *SettingsService) UpdateAudioBriefingSettings(ctx context.Context, userID string, in UpdateAudioBriefingSettingsInput) (*model.AudioBriefingSettings, error) {
	if s.audioBriefingRepo == nil {
		return nil, fmt.Errorf("audio briefing unavailable")
	}
	if in.IntervalHours != 3 && in.IntervalHours != 6 {
		return nil, fmt.Errorf("invalid interval_hours")
	}
	if in.ArticlesPerEpisode < 1 || in.ArticlesPerEpisode > 30 {
		return nil, fmt.Errorf("invalid articles_per_episode")
	}
	if in.TargetDurationMinutes < 5 || in.TargetDurationMinutes > 60 {
		return nil, fmt.Errorf("invalid target_duration_minutes")
	}
	return s.audioBriefingRepo.UpsertSettings(
		ctx,
		userID,
		in.Enabled,
		in.IntervalHours,
		in.ArticlesPerEpisode,
		in.TargetDurationMinutes,
		NormalizePersonaMode(in.DefaultPersonaMode),
		normalizeAudioBriefingDefaultPersona(in.DefaultPersona),
	)
}

func (s *SettingsService) UpdateAudioBriefingPersonaVoices(ctx context.Context, userID string, rows []UpdateAudioBriefingPersonaVoiceInput) ([]model.AudioBriefingPersonaVoice, error) {
	if s.audioBriefingRepo == nil {
		return nil, fmt.Errorf("audio briefing unavailable")
	}
	normalizedRows, err := validateAudioBriefingPersonaVoiceInputs(rows)
	if err != nil {
		return nil, err
	}
	if s.aivisModelRepo != nil {
		var aivisSnapshots []repository.AivisModelSnapshot
		needsAivisValidation := false
		for _, row := range normalizedRows {
			if strings.EqualFold(strings.TrimSpace(row.TTSProvider), "aivis") {
				needsAivisValidation = true
				break
			}
		}
		if needsAivisValidation {
			var latestRun *repository.AivisSyncRun
			aivisSnapshots, latestRun, err = s.aivisModelRepo.ListLatestSnapshots(ctx)
			if err != nil {
				return nil, err
			}
			if latestRun == nil || len(aivisSnapshots) == 0 {
				return nil, fmt.Errorf("aivis models are not synced")
			}
			for _, row := range normalizedRows {
				if !strings.EqualFold(strings.TrimSpace(row.TTSProvider), "aivis") {
					continue
				}
				if err := validateAivisVoiceSelectionAgainstSnapshots(aivisSnapshots, row.VoiceModel, row.VoiceStyle); err != nil {
					return nil, fmt.Errorf("invalid aivis voice for %s: %w", row.Persona, err)
				}
			}
		}
	}
	return s.audioBriefingRepo.UpsertPersonaVoices(ctx, userID, normalizedRows)
}

func normalizeOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func normalizePodcastLanguage(v *string) string {
	if v == nil {
		return "ja"
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return "ja"
	}
	return s
}

func generatePodcastFeedSlug() (string, error) {
	var buf [10]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "p_" + fmt.Sprintf("%x", buf[:]), nil
}

func (s *SettingsService) ensurePodcastFeedSlug(ctx context.Context, existing *string) (string, error) {
	if existing != nil && strings.TrimSpace(*existing) != "" {
		return strings.TrimSpace(*existing), nil
	}
	for i := 0; i < 8; i++ {
		slug, err := generatePodcastFeedSlug()
		if err != nil {
			return "", err
		}
		row, err := s.repo.GetByPodcastFeedSlug(ctx, slug)
		if err != nil {
			return "", err
		}
		if row == nil {
			return slug, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique podcast_feed_slug")
}

func (s *SettingsService) UpdatePodcastSettings(ctx context.Context, userID string, in UpdatePodcastSettingsInput) (*model.UserSettings, error) {
	settings, err := s.repo.EnsureDefaults(ctx, userID)
	if err != nil {
		return nil, err
	}
	slug, err := s.ensurePodcastFeedSlug(ctx, settings.PodcastFeedSlug)
	if err != nil {
		return nil, err
	}
	category, subcategory, err := normalizePodcastCategorySelection(in.Category, in.Subcategory)
	if err != nil {
		return nil, err
	}
	return s.repo.UpsertPodcastConfig(
		ctx,
		userID,
		in.Enabled,
		slug,
		normalizeOptionalString(in.Title),
		normalizeOptionalString(in.Description),
		normalizeOptionalString(in.Author),
		normalizePodcastLanguage(in.Language),
		category,
		subcategory,
		in.Explicit,
		normalizeOptionalString(in.ArtworkURL),
	)
}

func (s *SettingsService) UpdateObsidianExport(ctx context.Context, userID string, in UpdateObsidianExportInput) (*model.ObsidianExportSettings, error) {
	repoOwner := normalizeOptionalString(in.GitHubRepoOwner)
	repoName := normalizeOptionalString(in.GitHubRepoName)
	repoBranch := normalizeOptionalString(in.GitHubRepoBranch)
	vaultRootPath := normalizeOptionalString(in.VaultRootPath)
	keywordLinkMode := normalizeOptionalString(in.KeywordLinkMode)
	if keywordLinkMode != nil && *keywordLinkMode != "topics_only" {
		return nil, fmt.Errorf("invalid keyword_link_mode")
	}
	return s.obsidianRepo.UpsertConfig(ctx, userID, in.Enabled, repoOwner, repoName, repoBranch, vaultRootPath, keywordLinkMode)
}

func (s *SettingsService) UpsertObsidianGitHubInstallation(ctx context.Context, userID string, installationID int64) (*model.ObsidianExportSettings, error) {
	var owner *string
	if s.githubApp != nil && s.githubApp.Enabled() {
		installation, err := s.githubApp.GetInstallation(ctx, installationID)
		if err != nil {
			return nil, err
		}
		if installation != nil && installation.Account != nil && strings.TrimSpace(installation.Account.Login) != "" {
			v := strings.TrimSpace(installation.Account.Login)
			owner = &v
		}
	}
	return s.obsidianRepo.UpsertInstallation(ctx, userID, installationID, owner)
}

func (s *SettingsService) SetAPIKey(ctx context.Context, userID, provider, apiKey string) (*model.UserSettings, error) {
	if s.cipher == nil || !s.cipher.Enabled() {
		return nil, fmt.Errorf("user secret encryption is not configured")
	}
	key := strings.TrimSpace(apiKey)
	enc, err := s.cipher.EncryptString(key)
	if err != nil {
		return nil, err
	}
	last4 := key
	if len(last4) > 4 {
		last4 = last4[len(last4)-4:]
	}
	switch provider {
	case "anthropic":
		return s.repo.SetAnthropicAPIKey(ctx, userID, enc, last4)
	case "openai":
		return s.repo.SetOpenAIAPIKey(ctx, userID, enc, last4)
	case "google":
		return s.repo.SetGoogleAPIKey(ctx, userID, enc, last4)
	case "groq":
		return s.repo.SetGroqAPIKey(ctx, userID, enc, last4)
	case "deepseek":
		return s.repo.SetDeepSeekAPIKey(ctx, userID, enc, last4)
	case "alibaba":
		return s.repo.SetAlibabaAPIKey(ctx, userID, enc, last4)
	case "mistral":
		return s.repo.SetMistralAPIKey(ctx, userID, enc, last4)
	case "xai":
		return s.repo.SetXAIAPIKey(ctx, userID, enc, last4)
	case "zai":
		return s.repo.SetZAIAPIKey(ctx, userID, enc, last4)
	case "fireworks":
		return s.repo.SetFireworksAPIKey(ctx, userID, enc, last4)
	case "poe":
		return s.repo.SetPoeAPIKey(ctx, userID, enc, last4)
	case "openrouter":
		return s.repo.SetOpenRouterAPIKey(ctx, userID, enc, last4)
	case "aivis":
		return s.repo.SetAivisAPIKey(ctx, userID, enc, last4)
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}

func (s *SettingsService) DeleteAPIKey(ctx context.Context, userID, provider string) (*model.UserSettings, error) {
	switch provider {
	case "anthropic":
		return s.repo.ClearAnthropicAPIKey(ctx, userID)
	case "openai":
		return s.repo.ClearOpenAIAPIKey(ctx, userID)
	case "google":
		return s.repo.ClearGoogleAPIKey(ctx, userID)
	case "groq":
		return s.repo.ClearGroqAPIKey(ctx, userID)
	case "deepseek":
		return s.repo.ClearDeepSeekAPIKey(ctx, userID)
	case "alibaba":
		return s.repo.ClearAlibabaAPIKey(ctx, userID)
	case "mistral":
		return s.repo.ClearMistralAPIKey(ctx, userID)
	case "xai":
		return s.repo.ClearXAIAPIKey(ctx, userID)
	case "zai":
		return s.repo.ClearZAIAPIKey(ctx, userID)
	case "fireworks":
		return s.repo.ClearFireworksAPIKey(ctx, userID)
	case "poe":
		return s.repo.ClearPoeAPIKey(ctx, userID)
	case "openrouter":
		return s.repo.ClearOpenRouterAPIKey(ctx, userID)
	case "aivis":
		return s.repo.ClearAivisAPIKey(ctx, userID)
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}
