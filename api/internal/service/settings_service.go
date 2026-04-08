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
	repo                    *repository.UserSettingsRepo
	userRepo                *repository.UserRepo
	audioBriefingRepo       *repository.AudioBriefingRepo
	audioBriefingPresetRepo *repository.AudioBriefingPresetRepo
	summaryAudioRepo        *repository.SummaryAudioVoiceSettingsRepo
	aivisModelRepo          *repository.AivisModelRepo
	obsidianRepo            *repository.ObsidianExportRepo
	llmUsageRepo            *repository.LLMUsageLogRepo
	openRouterOverrideRepo  *repository.OpenRouterModelOverrideRepo
	uiFontCatalog           *UIFontCatalogService
	cipher                  *SecretCipher
	githubApp               *GitHubAppClient
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
	HasMoonshotAPIKey       bool             `json:"has_moonshot_api_key"`
	MoonshotAPIKeyLast4     *string          `json:"moonshot_api_key_last4,omitempty"`
	HasXAIAPIKey            bool             `json:"has_xai_api_key"`
	XAIAPIKeyLast4          *string          `json:"xai_api_key_last4,omitempty"`
	HasZAIAPIKey            bool             `json:"has_zai_api_key"`
	ZAIAPIKeyLast4          *string          `json:"zai_api_key_last4,omitempty"`
	HasFireworksAPIKey      bool             `json:"has_fireworks_api_key"`
	FireworksAPIKeyLast4    *string          `json:"fireworks_api_key_last4,omitempty"`
	HasPoeAPIKey            bool             `json:"has_poe_api_key"`
	PoeAPIKeyLast4          *string          `json:"poe_api_key_last4,omitempty"`
	HasSiliconFlowAPIKey    bool             `json:"has_siliconflow_api_key"`
	SiliconFlowAPIKeyLast4  *string          `json:"siliconflow_api_key_last4,omitempty"`
	HasOpenRouterAPIKey     bool             `json:"has_openrouter_api_key"`
	OpenRouterAPIKeyLast4   *string          `json:"openrouter_api_key_last4,omitempty"`
	HasAivisAPIKey          bool             `json:"has_aivis_api_key"`
	AivisAPIKeyLast4        *string          `json:"aivis_api_key_last4,omitempty"`
	HasFishAudioAPIKey      bool             `json:"has_fish_api_key"`
	FishAudioAPIKeyLast4    *string          `json:"fish_api_key_last4,omitempty"`
	HasElevenLabsAPIKey     bool             `json:"has_elevenlabs_api_key"`
	ElevenLabsAPIKeyLast4   *string          `json:"elevenlabs_api_key_last4,omitempty"`
	AivisUserDictionaryUUID *string          `json:"aivis_user_dictionary_uuid,omitempty"`
	GeminiTTSEnabled        bool             `json:"gemini_tts_enabled"`
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
	SummaryAudio            map[string]any   `json:"summary_audio"`
	UIFontSansKey           string           `json:"ui_font_sans_key"`
	UIFontSerifKey          string           `json:"ui_font_serif_key"`
	CurrentMonth            map[string]any   `json:"current_month"`
	ObsidianExport          map[string]any   `json:"obsidian_export"`
	NotificationPriority    map[string]any   `json:"notification_priority"`
}

type UpdateLLMModelsInput struct {
	Facts                       *string
	FactsSecondary              *string
	FactsSecondaryRatePercent   *int
	FactsFallback               *string
	Summary                     *string
	SummarySecondary            *string
	SummarySecondaryRatePercent *int
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
	AINavigatorBrief            *string
	AINavigatorBriefFallback    *string
	AudioBriefingScript         *string
	AudioBriefingScriptFallback *string
	TTSMarkupPreprocessModel    *string
}

type UpdateAudioBriefingSettingsInput struct {
	Enabled                     bool
	ScheduleMode                string
	IntervalHours               int
	ArticlesPerEpisode          int
	TargetDurationMinutes       int
	ChunkTrailingSilenceSeconds float64
	ProgramName                 *string
	DefaultPersonaMode          *string
	DefaultPersona              *string
	ConversationMode            *string
	BGMEnabled                  bool
	BGMR2Prefix                 *string
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
	Persona                  string  `json:"persona"`
	TTSProvider              string  `json:"tts_provider"`
	TTSModel                 string  `json:"tts_model"`
	VoiceModel               string  `json:"voice_model"`
	VoiceStyle               string  `json:"voice_style"`
	ProviderVoiceLabel       string  `json:"provider_voice_label"`
	ProviderVoiceDescription string  `json:"provider_voice_description"`
	SpeechRate               float64 `json:"speech_rate"`
	EmotionalIntensity       float64 `json:"emotional_intensity"`
	TempoDynamics            float64 `json:"tempo_dynamics"`
	LineBreakSilenceSeconds  float64 `json:"line_break_silence_seconds"`
	Pitch                    float64 `json:"pitch"`
	VolumeGain               float64 `json:"volume_gain"`
}

type SaveAudioBriefingPresetInput struct {
	Name               string                                 `json:"name"`
	DefaultPersonaMode string                                 `json:"default_persona_mode"`
	DefaultPersona     string                                 `json:"default_persona"`
	ConversationMode   string                                 `json:"conversation_mode"`
	Voices             []UpdateAudioBriefingPersonaVoiceInput `json:"voices"`
}

type UpdateSummaryAudioVoiceSettingsInput struct {
	TTSProvider              string
	TTSModel                 string
	VoiceModel               string
	VoiceStyle               string
	ProviderVoiceLabel       string
	ProviderVoiceDescription string
	SpeechRate               float64
	EmotionalIntensity       float64
	TempoDynamics            float64
	LineBreakSilenceSeconds  float64
	Pitch                    float64
	VolumeGain               float64
	AivisUserDictionaryUUID  *string
}

type UpdateUIFontSettingsInput struct {
	UIFontSansKey  string `json:"ui_font_sans_key"`
	UIFontSerifKey string `json:"ui_font_serif_key"`
}

var modelSettingPurposes = map[string]string{
	"facts":                          "facts",
	"facts_secondary":                "facts",
	"facts_fallback":                 "facts",
	"summary":                        "summary",
	"summary_secondary":              "summary",
	"summary_fallback":               "summary",
	"digest_cluster":                 "digest_cluster_draft",
	"digest":                         "digest",
	"ask":                            "ask",
	"source_suggestion":              "source_suggestion",
	"facts_check":                    "facts",
	"faithfulness_check":             "summary",
	"navigator":                      "summary",
	"navigator_fallback":             "summary",
	"ai_navigator_brief":             "summary",
	"ai_navigator_brief_fallback":    "summary",
	"audio_briefing_script":          "summary",
	"audio_briefing_script_fallback": "summary",
}

var modelSettingRequiredCapabilities = map[string][]string{
	"facts":                          {"structured_output"},
	"facts_secondary":                {"structured_output"},
	"facts_fallback":                 {"structured_output"},
	"summary":                        {"structured_output"},
	"summary_secondary":              {"structured_output"},
	"summary_fallback":               {"structured_output"},
	"digest_cluster":                 {"structured_output"},
	"digest":                         {"structured_output"},
	"ask":                            {"structured_output"},
	"source_suggestion":              {"structured_output"},
	"facts_check":                    {"structured_output"},
	"faithfulness_check":             {"structured_output"},
	"navigator":                      {"structured_output"},
	"navigator_fallback":             {"structured_output"},
	"ai_navigator_brief":             {"structured_output"},
	"ai_navigator_brief_fallback":    {"structured_output"},
	"audio_briefing_script":          {"structured_output"},
	"audio_briefing_script_fallback": {"structured_output"},
}

func NewSettingsService(repo *repository.UserSettingsRepo, userRepo *repository.UserRepo, audioBriefingRepo *repository.AudioBriefingRepo, summaryAudioRepo *repository.SummaryAudioVoiceSettingsRepo, aivisModelRepo *repository.AivisModelRepo, obsidianRepo *repository.ObsidianExportRepo, llmUsageRepo *repository.LLMUsageLogRepo, openRouterOverrideRepo *repository.OpenRouterModelOverrideRepo, cipher *SecretCipher, githubApp *GitHubAppClient) *SettingsService {
	return &SettingsService{
		repo:                    repo,
		userRepo:                userRepo,
		audioBriefingRepo:       audioBriefingRepo,
		audioBriefingPresetRepo: nil,
		summaryAudioRepo:        summaryAudioRepo,
		aivisModelRepo:          aivisModelRepo,
		obsidianRepo:            obsidianRepo,
		llmUsageRepo:            llmUsageRepo,
		openRouterOverrideRepo:  openRouterOverrideRepo,
		uiFontCatalog:           NewUIFontCatalogService(),
		cipher:                  cipher,
		githubApp:               githubApp,
	}
}

func (s *SettingsService) UserRepo() *repository.UserRepo {
	if s == nil {
		return nil
	}
	return s.userRepo
}

func (s *SettingsService) SetAudioBriefingPresetRepo(repo *repository.AudioBriefingPresetRepo) {
	if s == nil {
		return
	}
	s.audioBriefingPresetRepo = repo
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
		"facts_secondary":                settings.FactsSecondaryModel,
		"facts_secondary_rate_percent":   settings.FactsSecondaryRatePercent,
		"facts_fallback":                 settings.FactsFallbackModel,
		"summary":                        settings.SummaryModel,
		"summary_secondary":              settings.SummarySecondaryModel,
		"summary_secondary_rate_percent": settings.SummarySecondaryRatePercent,
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
		"ai_navigator_brief":             settings.AINavigatorBriefModel,
		"ai_navigator_brief_fallback":    settings.AINavigatorBriefFallbackModel,
		"audio_briefing_script":          settings.AudioBriefingScriptModel,
		"audio_briefing_script_fallback": settings.AudioBriefingScriptFallbackModel,
		"tts_markup_preprocess_model":    settings.TTSMarkupPreprocessModel,
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
			"enabled":                        false,
			"schedule_mode":                  AudioBriefingScheduleModeInterval,
			"interval_hours":                 6,
			"articles_per_episode":           5,
			"target_duration_minutes":        20,
			"chunk_trailing_silence_seconds": 1.0,
			"program_name":                   nil,
			"default_persona_mode":           PersonaModeFixed,
			"default_persona":                "editor",
			"conversation_mode":              "single",
			"bgm_enabled":                    false,
			"bgm_r2_prefix":                  nil,
		}
	}
	return map[string]any{
		"enabled":                        settings.Enabled,
		"schedule_mode":                  NormalizeAudioBriefingScheduleMode(settings.ScheduleMode),
		"interval_hours":                 settings.IntervalHours,
		"articles_per_episode":           settings.ArticlesPerEpisode,
		"target_duration_minutes":        settings.TargetDurationMinutes,
		"chunk_trailing_silence_seconds": settings.ChunkTrailingSilenceSeconds,
		"program_name":                   settings.ProgramName,
		"default_persona_mode":           NormalizePersonaMode(&settings.DefaultPersonaMode),
		"default_persona":                settings.DefaultPersona,
		"conversation_mode":              normalizeAudioBriefingConversationMode(&settings.ConversationMode),
		"bgm_enabled":                    settings.BGMEnabled,
		"bgm_r2_prefix":                  settings.BGMR2Prefix,
	}
}

func AudioBriefingPersonaVoicesPayload(rows []model.AudioBriefingPersonaVoice) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"persona":                    row.Persona,
			"tts_provider":               row.TTSProvider,
			"tts_model":                  row.TTSModel,
			"voice_model":                row.VoiceModel,
			"voice_style":                row.VoiceStyle,
			"provider_voice_label":       row.ProviderVoiceLabel,
			"provider_voice_description": row.ProviderVoiceDescription,
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

func AudioBriefingPresetPayload(p model.AudioBriefingPreset) map[string]any {
	return map[string]any{
		"id":                   p.ID,
		"name":                 p.Name,
		"default_persona_mode": NormalizePersonaMode(&p.DefaultPersonaMode),
		"default_persona":      p.DefaultPersona,
		"conversation_mode":    normalizeAudioBriefingConversationMode(&p.ConversationMode),
		"voices":               AudioBriefingPersonaVoicesPayload(p.Voices),
		"created_at":           p.CreatedAt,
		"updated_at":           p.UpdatedAt,
	}
}

func SummaryAudioVoiceSettingsPayload(settings *model.SummaryAudioVoiceSettings) map[string]any {
	if settings == nil {
		return map[string]any{
			"tts_provider":               "",
			"tts_model":                  "",
			"voice_model":                "",
			"voice_style":                "",
			"provider_voice_label":       "",
			"provider_voice_description": "",
			"speech_rate":                0,
			"emotional_intensity":        0,
			"tempo_dynamics":             0,
			"line_break_silence_seconds": 0,
			"pitch":                      0,
			"volume_gain":                0,
			"aivis_user_dictionary_uuid": nil,
		}
	}
	return map[string]any{
		"tts_provider":               settings.TTSProvider,
		"tts_model":                  settings.TTSModel,
		"voice_model":                settings.VoiceModel,
		"voice_style":                settings.VoiceStyle,
		"provider_voice_label":       settings.ProviderVoiceLabel,
		"provider_voice_description": settings.ProviderVoiceDescription,
		"speech_rate":                settings.SpeechRate,
		"emotional_intensity":        settings.EmotionalIntensity,
		"tempo_dynamics":             settings.TempoDynamics,
		"line_break_silence_seconds": settings.LineBreakSilenceSeconds,
		"pitch":                      settings.Pitch,
		"volume_gain":                settings.VolumeGain,
		"aivis_user_dictionary_uuid": settings.AivisUserDictionaryUUID,
	}
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
	var summaryAudioSettings *model.SummaryAudioVoiceSettings
	if s.summaryAudioRepo != nil {
		summaryAudioSettings, err = s.summaryAudioRepo.EnsureDefaults(ctx, userID)
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
		HasMoonshotAPIKey:       settings.HasMoonshotAPIKey,
		MoonshotAPIKeyLast4:     settings.MoonshotAPIKeyLast4,
		HasXAIAPIKey:            settings.HasXAIAPIKey,
		XAIAPIKeyLast4:          settings.XAIAPIKeyLast4,
		HasZAIAPIKey:            settings.HasZAIAPIKey,
		ZAIAPIKeyLast4:          settings.ZAIAPIKeyLast4,
		HasFireworksAPIKey:      settings.HasFireworksAPIKey,
		FireworksAPIKeyLast4:    settings.FireworksAPIKeyLast4,
		HasPoeAPIKey:            settings.HasPoeAPIKey,
		PoeAPIKeyLast4:          settings.PoeAPIKeyLast4,
		HasSiliconFlowAPIKey:    settings.HasSiliconFlowAPIKey,
		SiliconFlowAPIKeyLast4:  settings.SiliconFlowAPIKeyLast4,
		HasOpenRouterAPIKey:     settings.HasOpenRouterAPIKey,
		OpenRouterAPIKeyLast4:   settings.OpenRouterAPIKeyLast4,
		HasAivisAPIKey:          settings.HasAivisAPIKey,
		AivisAPIKeyLast4:        settings.AivisAPIKeyLast4,
		HasFishAudioAPIKey:      settings.HasFishAudioAPIKey,
		FishAudioAPIKeyLast4:    settings.FishAudioAPIKeyLast4,
		HasElevenLabsAPIKey:     settings.HasElevenLabsAPIKey,
		ElevenLabsAPIKeyLast4:   settings.ElevenLabsAPIKeyLast4,
		AivisUserDictionaryUUID: settings.AivisUserDictionaryUUID,
		GeminiTTSEnabled:        GeminiTTSEnabledForUser(ctx, s.userRepo, userID),
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
		SummaryAudio:            SummaryAudioVoiceSettingsPayload(summaryAudioSettings),
		UIFontSansKey:           normalizeUIFontKeyOrDefault(settings.UIFontSansKey, DefaultUIFontSansKey),
		UIFontSerifKey:          normalizeUIFontKeyOrDefault(settings.UIFontSerifKey, DefaultUIFontSerifKey),
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

func normalizeUIFontKeyOrDefault(raw, fallback string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "" {
		return fallback
	}
	return normalized
}

func (s *SettingsService) LoadUIFontCatalog(ctx context.Context) (*UIFontCatalog, error) {
	if s.uiFontCatalog == nil {
		return nil, fmt.Errorf("ui font catalog unavailable")
	}
	return s.uiFontCatalog.LoadCatalog(ctx)
}

func (s *SettingsService) UpdateUIFontSettings(ctx context.Context, userID string, in UpdateUIFontSettingsInput) (*model.UserSettings, error) {
	catalog, err := s.LoadUIFontCatalog(ctx)
	if err != nil {
		return nil, err
	}
	sansKey := normalizeUIFontKeyOrDefault(in.UIFontSansKey, DefaultUIFontSansKey)
	serifKey := normalizeUIFontKeyOrDefault(in.UIFontSerifKey, DefaultUIFontSerifKey)
	if err := ValidateUIFontSelection(catalog, sansKey, serifKey); err != nil {
		return nil, err
	}
	return s.repo.UpsertUIFontConfig(ctx, userID, sansKey, serifKey)
}

func (s *SettingsService) GetSummaryAudioVoiceSettings(ctx context.Context, userID string) (*model.SummaryAudioVoiceSettings, error) {
	if s.summaryAudioRepo == nil {
		return nil, fmt.Errorf("summary audio unavailable")
	}
	return s.summaryAudioRepo.EnsureDefaults(ctx, userID)
}

func (s *SettingsService) UpdateSummaryAudioVoiceSettings(ctx context.Context, userID string, in UpdateSummaryAudioVoiceSettingsInput) (*model.SummaryAudioVoiceSettings, error) {
	if s.summaryAudioRepo == nil {
		return nil, fmt.Errorf("summary audio unavailable")
	}
	normalized, err := normalizeSummaryAudioVoiceSettingsInput(in)
	if err != nil {
		return nil, err
	}
	if s.aivisModelRepo != nil && strings.EqualFold(strings.TrimSpace(normalized.TTSProvider), "aivis") {
		snapshots, latestRun, err := s.aivisModelRepo.ListLatestSnapshots(ctx)
		if err != nil {
			return nil, err
		}
		if latestRun == nil || len(snapshots) == 0 {
			return nil, fmt.Errorf("aivis models are not synced")
		}
		if err := validateAivisVoiceSelectionAgainstSnapshots(snapshots, normalized.VoiceModel, normalized.VoiceStyle); err != nil {
			return nil, fmt.Errorf("invalid aivis voice for summary_audio: %w", err)
		}
	}
	return s.summaryAudioRepo.Upsert(ctx, model.SummaryAudioVoiceSettings{
		UserID:                   userID,
		TTSProvider:              normalized.TTSProvider,
		TTSModel:                 normalized.TTSModel,
		VoiceModel:               normalized.VoiceModel,
		VoiceStyle:               normalized.VoiceStyle,
		ProviderVoiceLabel:       normalized.ProviderVoiceLabel,
		ProviderVoiceDescription: normalized.ProviderVoiceDescription,
		SpeechRate:               normalized.SpeechRate,
		EmotionalIntensity:       normalized.EmotionalIntensity,
		TempoDynamics:            normalized.TempoDynamics,
		LineBreakSilenceSeconds:  normalized.LineBreakSilenceSeconds,
		Pitch:                    normalized.Pitch,
		VolumeGain:               normalized.VolumeGain,
		AivisUserDictionaryUUID:  normalized.AivisUserDictionaryUUID,
	})
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
		provider := strings.TrimSpace(strings.ToLower(row.TTSProvider))
		caps := LookupTTSProviderCapabilities(provider)
		providerVoiceLabel := strings.TrimSpace(row.ProviderVoiceLabel)
		providerVoiceDescription := strings.TrimSpace(row.ProviderVoiceDescription)
		switch provider {
		case "aivis", "mock", "xai", "openai", "gemini_tts", "fish", "elevenlabs":
		default:
			return nil, fmt.Errorf("invalid tts_provider for %s", persona)
		}
		voiceModel := strings.TrimSpace(row.VoiceModel)
		voiceStyle := strings.TrimSpace(row.VoiceStyle)
		ttsModel := strings.TrimSpace(row.TTSModel)
		// Allow incomplete persona rows to remain unset in the UI without persisting
		// placeholder records. Only partially configured rows are rejected.
		if voiceModel == "" && voiceStyle == "" && ttsModel == "" {
			continue
		}
		if voiceModel == "" {
			return nil, fmt.Errorf("invalid voice_model for %s", persona)
		}
		if caps.SupportsSeparateTTSModel && ttsModel == "" {
			return nil, fmt.Errorf("invalid tts_model for %s", persona)
		}
		if caps.RequiresVoiceStyle && voiceStyle == "" {
			return nil, fmt.Errorf("invalid voice_style for %s", persona)
		}
		if caps.SupportsSpeechTuning && (row.SpeechRate != 0 || provider == "aivis") {
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
		}
		out = append(out, model.AudioBriefingPersonaVoice{
			Persona:                  persona,
			TTSProvider:              provider,
			TTSModel:                 ttsModel,
			VoiceModel:               voiceModel,
			VoiceStyle:               voiceStyle,
			ProviderVoiceLabel:       providerVoiceLabel,
			ProviderVoiceDescription: providerVoiceDescription,
			SpeechRate:               row.SpeechRate,
			EmotionalIntensity:       row.EmotionalIntensity,
			TempoDynamics:            row.TempoDynamics,
			LineBreakSilenceSeconds:  row.LineBreakSilenceSeconds,
			Pitch:                    row.Pitch,
			VolumeGain:               row.VolumeGain,
		})
	}
	return out, nil
}

func (s *SettingsService) validateAudioBriefingPersonaVoiceInputsWithAivis(ctx context.Context, rows []UpdateAudioBriefingPersonaVoiceInput) ([]model.AudioBriefingPersonaVoice, error) {
	normalizedRows, err := validateAudioBriefingPersonaVoiceInputs(rows)
	if err != nil {
		return nil, err
	}
	if s.aivisModelRepo == nil {
		return normalizedRows, nil
	}
	needsAivisValidation := false
	for _, row := range normalizedRows {
		if strings.EqualFold(strings.TrimSpace(row.TTSProvider), "aivis") {
			needsAivisValidation = true
			break
		}
	}
	if !needsAivisValidation {
		return normalizedRows, nil
	}
	snapshots, latestRun, err := s.aivisModelRepo.ListLatestSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	if latestRun == nil || len(snapshots) == 0 {
		return nil, fmt.Errorf("aivis models are not synced")
	}
	for _, row := range normalizedRows {
		if !strings.EqualFold(strings.TrimSpace(row.TTSProvider), "aivis") {
			continue
		}
		if err := validateAivisVoiceSelectionAgainstSnapshots(snapshots, row.VoiceModel, row.VoiceStyle); err != nil {
			return nil, fmt.Errorf("invalid aivis voice for %s: %w", row.Persona, err)
		}
	}
	return normalizedRows, nil
}

func normalizeSummaryAudioVoiceSettingsInput(in UpdateSummaryAudioVoiceSettingsInput) (*model.SummaryAudioVoiceSettings, error) {
	provider := strings.TrimSpace(strings.ToLower(in.TTSProvider))
	voiceModel := strings.TrimSpace(in.VoiceModel)
	voiceStyle := strings.TrimSpace(in.VoiceStyle)
	ttsModel := strings.TrimSpace(in.TTSModel)
	providerVoiceLabel := strings.TrimSpace(in.ProviderVoiceLabel)
	providerVoiceDescription := strings.TrimSpace(in.ProviderVoiceDescription)
	dictUUID := normalizeOptionalString(in.AivisUserDictionaryUUID)
	if provider == "" && voiceModel == "" && voiceStyle == "" && ttsModel == "" && dictUUID == nil &&
		in.SpeechRate == 0 && in.EmotionalIntensity == 0 && in.TempoDynamics == 0 && in.LineBreakSilenceSeconds == 0 &&
		in.Pitch == 0 && in.VolumeGain == 0 {
		return &model.SummaryAudioVoiceSettings{}, nil
	}
	switch provider {
	case "aivis", "mock", "xai", "openai", "gemini_tts", "fish", "elevenlabs":
	default:
		return nil, fmt.Errorf("invalid tts_provider")
	}
	if voiceModel == "" {
		return nil, fmt.Errorf("invalid voice_model")
	}
	caps := LookupTTSProviderCapabilities(provider)
	if caps.SupportsSeparateTTSModel && ttsModel == "" {
		return nil, fmt.Errorf("invalid tts_model")
	}
	if caps.RequiresVoiceStyle && voiceStyle == "" {
		return nil, fmt.Errorf("invalid voice_style")
	}
	if caps.SupportsSpeechTuning && (in.SpeechRate != 0 || provider == "aivis") {
		if in.SpeechRate < 0.5 || in.SpeechRate > 2.0 {
			return nil, fmt.Errorf("invalid speech_rate")
		}
		if in.EmotionalIntensity < 0 || in.EmotionalIntensity > 2.0 {
			return nil, fmt.Errorf("invalid emotional_intensity")
		}
		if in.TempoDynamics < 0 || in.TempoDynamics > 2.0 {
			return nil, fmt.Errorf("invalid tempo_dynamics")
		}
		if in.LineBreakSilenceSeconds < 0 || in.LineBreakSilenceSeconds > 5.0 {
			return nil, fmt.Errorf("invalid line_break_silence_seconds")
		}
		if in.Pitch < -12 || in.Pitch > 12 {
			return nil, fmt.Errorf("invalid pitch")
		}
		if in.VolumeGain < -24 || in.VolumeGain > 24 {
			return nil, fmt.Errorf("invalid volume_gain")
		}
	}
	return &model.SummaryAudioVoiceSettings{
		TTSProvider:              provider,
		TTSModel:                 ttsModel,
		VoiceModel:               voiceModel,
		VoiceStyle:               voiceStyle,
		ProviderVoiceLabel:       providerVoiceLabel,
		ProviderVoiceDescription: providerVoiceDescription,
		SpeechRate:               in.SpeechRate,
		EmotionalIntensity:       in.EmotionalIntensity,
		TempoDynamics:            in.TempoDynamics,
		LineBreakSilenceSeconds:  in.LineBreakSilenceSeconds,
		Pitch:                    in.Pitch,
		VolumeGain:               in.VolumeGain,
		AivisUserDictionaryUUID:  dictUUID,
	}, nil
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

func validateFishAudioVoiceSelectionAgainstSnapshots(snapshots []repository.FishAudioModelSnapshot, voiceModel string) error {
	modelID := strings.TrimSpace(voiceModel)
	if modelID == "" {
		return fmt.Errorf("voice model is empty")
	}
	for _, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.ModelID) == modelID {
			return nil
		}
	}
	return fmt.Errorf("fish model is not synced")
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

func validateCatalogChatModel(catalog *LLMCatalog, model *string, settingKey string) error {
	if model == nil {
		return nil
	}
	if CatalogChatModelByIDInCatalog(catalog, *model) == nil {
		return fmt.Errorf("invalid model for %s", settingKey)
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
		"facts_secondary":                normalizeOptionalModel(in.FactsSecondary),
		"facts_fallback":                 normalizeOptionalModel(in.FactsFallback),
		"summary":                        normalizeOptionalModel(in.Summary),
		"summary_secondary":              normalizeOptionalModel(in.SummarySecondary),
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
		"ai_navigator_brief":             normalizeOptionalModel(in.AINavigatorBrief),
		"ai_navigator_brief_fallback":    normalizeOptionalModel(in.AINavigatorBriefFallback),
		"audio_briefing_script":          normalizeOptionalModel(in.AudioBriefingScript),
		"audio_briefing_script_fallback": normalizeOptionalModel(in.AudioBriefingScriptFallback),
		"tts_markup_preprocess_model":    normalizeOptionalModel(in.TTSMarkupPreprocessModel),
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
	if err := validateCatalogChatModel(catalog, normalized["tts_markup_preprocess_model"], "tts_markup_preprocess_model"); err != nil {
		return nil, err
	}
	return s.repo.UpsertLLMModelConfig(
		ctx,
		userID,
		normalized["facts"],
		normalized["facts_secondary"],
		normalizeModelSplitRatePercent(in.FactsSecondaryRatePercent),
		normalized["facts_fallback"],
		normalized["summary"],
		normalized["summary_secondary"],
		normalizeModelSplitRatePercent(in.SummarySecondaryRatePercent),
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
		normalized["ai_navigator_brief"],
		normalized["ai_navigator_brief_fallback"],
		normalized["audio_briefing_script"],
		normalized["audio_briefing_script_fallback"],
		normalized["tts_markup_preprocess_model"],
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
	rawScheduleMode := strings.TrimSpace(strings.ToLower(in.ScheduleMode))
	switch rawScheduleMode {
	case "", AudioBriefingScheduleModeInterval:
		if in.IntervalHours != 3 && in.IntervalHours != 6 {
			return nil, fmt.Errorf("invalid interval_hours")
		}
		rawScheduleMode = AudioBriefingScheduleModeInterval
	case AudioBriefingScheduleModeFixedSlots3x:
		if in.IntervalHours != 3 && in.IntervalHours != 6 {
			in.IntervalHours = 6
		}
	default:
		return nil, fmt.Errorf("invalid schedule_mode")
	}
	scheduleMode := NormalizeAudioBriefingScheduleMode(rawScheduleMode)
	if in.ArticlesPerEpisode < 1 || in.ArticlesPerEpisode > 30 {
		return nil, fmt.Errorf("invalid articles_per_episode")
	}
	if in.TargetDurationMinutes < 5 || in.TargetDurationMinutes > 60 {
		return nil, fmt.Errorf("invalid target_duration_minutes")
	}
	if in.ChunkTrailingSilenceSeconds < 0 || in.ChunkTrailingSilenceSeconds > 5 {
		return nil, fmt.Errorf("invalid chunk_trailing_silence_seconds")
	}
	programName := normalizeAudioBriefingProgramName(in.ProgramName)
	if programName != nil && len([]rune(*programName)) > 120 {
		return nil, fmt.Errorf("invalid program_name")
	}
	bgmPrefix := normalizeOptionalString(in.BGMR2Prefix)
	if in.BGMEnabled && bgmPrefix == nil {
		return nil, fmt.Errorf("invalid bgm_r2_prefix")
	}
	return s.audioBriefingRepo.UpsertSettings(
		ctx,
		userID,
		in.Enabled,
		scheduleMode,
		in.IntervalHours,
		in.ArticlesPerEpisode,
		in.TargetDurationMinutes,
		in.ChunkTrailingSilenceSeconds,
		programName,
		NormalizePersonaMode(in.DefaultPersonaMode),
		normalizeAudioBriefingDefaultPersona(in.DefaultPersona),
		normalizeAudioBriefingConversationMode(in.ConversationMode),
		in.BGMEnabled,
		bgmPrefix,
	)
}

func normalizeAudioBriefingConversationMode(v *string) string {
	if v == nil {
		return "single"
	}
	switch strings.TrimSpace(strings.ToLower(*v)) {
	case "duo":
		return "duo"
	default:
		return "single"
	}
}

func normalizeAudioBriefingProgramName(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func (s *SettingsService) UpdateAudioBriefingPersonaVoices(ctx context.Context, userID string, rows []UpdateAudioBriefingPersonaVoiceInput) ([]model.AudioBriefingPersonaVoice, error) {
	if s.audioBriefingRepo == nil {
		return nil, fmt.Errorf("audio briefing unavailable")
	}
	normalizedRows, err := s.validateAudioBriefingPersonaVoiceInputsWithAivis(ctx, rows)
	if err != nil {
		return nil, err
	}
	return s.audioBriefingRepo.UpsertPersonaVoices(ctx, userID, normalizedRows)
}

func (s *SettingsService) ListAudioBriefingPresets(ctx context.Context, userID string) ([]model.AudioBriefingPreset, error) {
	if s.audioBriefingPresetRepo == nil {
		return nil, fmt.Errorf("audio briefing presets unavailable")
	}
	return s.audioBriefingPresetRepo.ListByUser(ctx, userID)
}

func (s *SettingsService) CreateAudioBriefingPreset(ctx context.Context, userID string, in SaveAudioBriefingPresetInput) (*model.AudioBriefingPreset, error) {
	if s.audioBriefingPresetRepo == nil {
		return nil, fmt.Errorf("audio briefing presets unavailable")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("preset name is required")
	}
	voices, err := s.validateAudioBriefingPersonaVoiceInputsWithAivis(ctx, in.Voices)
	if err != nil {
		return nil, err
	}
	preset, err := s.audioBriefingPresetRepo.Create(ctx, model.AudioBriefingPreset{
		UserID:             userID,
		Name:               name,
		DefaultPersonaMode: NormalizePersonaMode(&in.DefaultPersonaMode),
		DefaultPersona:     normalizeAudioBriefingDefaultPersona(&in.DefaultPersona),
		ConversationMode:   normalizeAudioBriefingConversationMode(&in.ConversationMode),
		Voices:             voices,
	})
	if err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *SettingsService) UpdateAudioBriefingPreset(ctx context.Context, userID, presetID string, in SaveAudioBriefingPresetInput) (*model.AudioBriefingPreset, error) {
	if s.audioBriefingPresetRepo == nil {
		return nil, fmt.Errorf("audio briefing presets unavailable")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("preset name is required")
	}
	voices, err := s.validateAudioBriefingPersonaVoiceInputsWithAivis(ctx, in.Voices)
	if err != nil {
		return nil, err
	}
	preset, err := s.audioBriefingPresetRepo.Update(ctx, model.AudioBriefingPreset{
		ID:                 presetID,
		UserID:             userID,
		Name:               name,
		DefaultPersonaMode: NormalizePersonaMode(&in.DefaultPersonaMode),
		DefaultPersona:     normalizeAudioBriefingDefaultPersona(&in.DefaultPersona),
		ConversationMode:   normalizeAudioBriefingConversationMode(&in.ConversationMode),
		Voices:             voices,
	})
	if err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *SettingsService) DeleteAudioBriefingPreset(ctx context.Context, userID, presetID string) error {
	if s.audioBriefingPresetRepo == nil {
		return fmt.Errorf("audio briefing presets unavailable")
	}
	return s.audioBriefingPresetRepo.Delete(ctx, userID, presetID)
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
	case "moonshot":
		return s.repo.SetMoonshotAPIKey(ctx, userID, enc, last4)
	case "xai":
		return s.repo.SetXAIAPIKey(ctx, userID, enc, last4)
	case "zai":
		return s.repo.SetZAIAPIKey(ctx, userID, enc, last4)
	case "fireworks":
		return s.repo.SetFireworksAPIKey(ctx, userID, enc, last4)
	case "poe":
		return s.repo.SetPoeAPIKey(ctx, userID, enc, last4)
	case "siliconflow":
		return s.repo.SetSiliconFlowAPIKey(ctx, userID, enc, last4)
	case "openrouter":
		return s.repo.SetOpenRouterAPIKey(ctx, userID, enc, last4)
	case "aivis":
		return s.repo.SetAivisAPIKey(ctx, userID, enc, last4)
	case "fish":
		return s.repo.SetFishAudioAPIKey(ctx, userID, enc, last4)
	case "elevenlabs":
		return s.repo.SetElevenLabsAPIKey(ctx, userID, enc, last4)
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
	case "moonshot":
		return s.repo.ClearMoonshotAPIKey(ctx, userID)
	case "xai":
		return s.repo.ClearXAIAPIKey(ctx, userID)
	case "zai":
		return s.repo.ClearZAIAPIKey(ctx, userID)
	case "fireworks":
		return s.repo.ClearFireworksAPIKey(ctx, userID)
	case "poe":
		return s.repo.ClearPoeAPIKey(ctx, userID)
	case "siliconflow":
		return s.repo.ClearSiliconFlowAPIKey(ctx, userID)
	case "openrouter":
		return s.repo.ClearOpenRouterAPIKey(ctx, userID)
	case "aivis":
		return s.repo.ClearAivisAPIKey(ctx, userID)
	case "fish":
		return s.repo.ClearFishAudioAPIKey(ctx, userID)
	case "elevenlabs":
		return s.repo.ClearElevenLabsAPIKey(ctx, userID)
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}
