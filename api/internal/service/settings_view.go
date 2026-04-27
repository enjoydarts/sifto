package service

import (
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type LLMModelsView struct {
	Facts                       *string `json:"facts"`
	FactsSecondary              *string `json:"facts_secondary"`
	FactsSecondaryRatePercent   int     `json:"facts_secondary_rate_percent"`
	FactsFallback               *string `json:"facts_fallback"`
	Summary                     *string `json:"summary"`
	SummarySecondary            *string `json:"summary_secondary"`
	SummarySecondaryRatePercent int     `json:"summary_secondary_rate_percent"`
	SummaryFallback             *string `json:"summary_fallback"`
	DigestCluster               *string `json:"digest_cluster"`
	Digest                      *string `json:"digest"`
	Ask                         *string `json:"ask"`
	SourceSuggestion            *string `json:"source_suggestion"`
	Embedding                   *string `json:"embedding"`
	FactsCheck                  *string `json:"facts_check"`
	FactsCheckFallback          *string `json:"facts_check_fallback"`
	FaithfulnessCheck           *string `json:"faithfulness_check"`
	FaithfulnessCheckFallback   *string `json:"faithfulness_check_fallback"`
	NavigatorEnabled            bool    `json:"navigator_enabled"`
	AINavigatorBriefEnabled     bool    `json:"ai_navigator_brief_enabled"`
	NavigatorPersonaMode        string  `json:"navigator_persona_mode"`
	NavigatorPersona            string  `json:"navigator_persona"`
	Navigator                   *string `json:"navigator"`
	NavigatorFallback           *string `json:"navigator_fallback"`
	AINavigatorBrief            *string `json:"ai_navigator_brief"`
	AINavigatorBriefFallback    *string `json:"ai_navigator_brief_fallback"`
	AudioBriefingScript         *string `json:"audio_briefing_script"`
	AudioBriefingScriptFallback *string `json:"audio_briefing_script_fallback"`
	TTSMarkupPreprocessModel    *string `json:"tts_markup_preprocess_model"`
}

type ReadingPlanView struct {
	Window          string `json:"window"`
	Size            int    `json:"size"`
	DiversifyTopics bool   `json:"diversify_topics"`
	ExcludeRead     bool   `json:"exclude_read"`
}

type PodcastView struct {
	Enabled             bool                        `json:"enabled"`
	FeedSlug            *string                     `json:"feed_slug"`
	RSSURL              *string                     `json:"rss_url"`
	Title               *string                     `json:"title"`
	Description         *string                     `json:"description"`
	Author              *string                     `json:"author"`
	Language            string                      `json:"language"`
	Category            *string                     `json:"category"`
	Subcategory         *string                     `json:"subcategory"`
	AvailableCategories []PodcastCategoryDefinition `json:"available_categories"`
	Explicit            bool                        `json:"explicit"`
	ArtworkURL          *string                     `json:"artwork_url"`
}

type AudioBriefingView struct {
	Enabled                     bool    `json:"enabled"`
	ScheduleMode                string  `json:"schedule_mode"`
	IntervalHours               int     `json:"interval_hours"`
	ArticlesPerEpisode          int     `json:"articles_per_episode"`
	TargetDurationMinutes       int     `json:"target_duration_minutes"`
	ChunkTrailingSilenceSeconds float64 `json:"chunk_trailing_silence_seconds"`
	ProgramName                 *string `json:"program_name"`
	DefaultPersonaMode          string  `json:"default_persona_mode"`
	DefaultPersona              string  `json:"default_persona"`
	ConversationMode            string  `json:"conversation_mode"`
	BGMEnabled                  bool    `json:"bgm_enabled"`
	BGMR2Prefix                 *string `json:"bgm_r2_prefix"`
}

type AudioBriefingPersonaVoiceView struct {
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

type AudioBriefingPresetView struct {
	ID                 string                          `json:"id"`
	Name               string                          `json:"name"`
	DefaultPersonaMode string                          `json:"default_persona_mode"`
	DefaultPersona     string                          `json:"default_persona"`
	ConversationMode   string                          `json:"conversation_mode"`
	Voices             []AudioBriefingPersonaVoiceView `json:"voices"`
	CreatedAt          time.Time                       `json:"created_at"`
	UpdatedAt          time.Time                       `json:"updated_at"`
}

type SummaryAudioView struct {
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
	AivisUserDictionaryUUID  *string `json:"aivis_user_dictionary_uuid"`
}

type ObsidianExportView struct {
	Enabled              bool       `json:"enabled"`
	GitHubInstallationID *int64     `json:"github_installation_id"`
	GitHubRepoOwner      *string    `json:"github_repo_owner"`
	GitHubRepoName       *string    `json:"github_repo_name"`
	GitHubRepoBranch     string     `json:"github_repo_branch"`
	VaultRootPath        *string    `json:"vault_root_path"`
	KeywordLinkMode      string     `json:"keyword_link_mode"`
	LastRunAt            *time.Time `json:"last_run_at"`
	LastSuccessAt        *time.Time `json:"last_success_at"`
	GitHubAppEnabled     *bool      `json:"github_app_enabled,omitempty"`
	GitHubAppInstallURL  *string    `json:"github_app_install_url,omitempty"`
}

type NotificationPriorityView struct {
	ID               string  `json:"id,omitempty"`
	Sensitivity      string  `json:"sensitivity"`
	DailyCap         int     `json:"daily_cap"`
	ThemeWeight      float64 `json:"theme_weight"`
	ImmediateEnabled bool    `json:"immediate_enabled"`
	BriefingEnabled  bool    `json:"briefing_enabled"`
	ReviewEnabled    bool    `json:"review_enabled"`
	GoalMatchEnabled bool    `json:"goal_match_enabled"`
}

type CurrentMonthView struct {
	MonthJST           string   `json:"month_jst"`
	PeriodStartJST     string   `json:"period_start_jst"`
	PeriodEndJST       string   `json:"period_end_jst"`
	EstimatedCostUSD   float64  `json:"estimated_cost_usd"`
	RemainingBudgetUSD *float64 `json:"remaining_budget_usd"`
	RemainingBudgetPct *float64 `json:"remaining_budget_pct"`
}

func NewLLMModelsView(settings *model.UserSettings) LLMModelsView {
	return LLMModelsView{
		Facts:                       settings.FactsModel,
		FactsSecondary:              settings.FactsSecondaryModel,
		FactsSecondaryRatePercent:   settings.FactsSecondaryRatePercent,
		FactsFallback:               settings.FactsFallbackModel,
		Summary:                     settings.SummaryModel,
		SummarySecondary:            settings.SummarySecondaryModel,
		SummarySecondaryRatePercent: settings.SummarySecondaryRatePercent,
		SummaryFallback:             settings.SummaryFallbackModel,
		DigestCluster:               settings.DigestClusterModel,
		Digest:                      settings.DigestModel,
		Ask:                         settings.AskModel,
		SourceSuggestion:            settings.SourceSuggestionModel,
		Embedding:                   settings.EmbeddingModel,
		FactsCheck:                  settings.FactsCheckModel,
		FactsCheckFallback:          settings.FactsCheckFallbackModel,
		FaithfulnessCheck:           settings.FaithfulnessCheckModel,
		FaithfulnessCheckFallback:   settings.FaithfulnessCheckFallbackModel,
		NavigatorEnabled:            settings.NavigatorEnabled,
		AINavigatorBriefEnabled:     settings.AINavigatorBriefEnabled,
		NavigatorPersonaMode:        NormalizePersonaMode(&settings.NavigatorPersonaMode),
		NavigatorPersona:            settings.NavigatorPersona,
		Navigator:                   settings.NavigatorModel,
		NavigatorFallback:           settings.NavigatorFallbackModel,
		AINavigatorBrief:            settings.AINavigatorBriefModel,
		AINavigatorBriefFallback:    settings.AINavigatorBriefFallbackModel,
		AudioBriefingScript:         settings.AudioBriefingScriptModel,
		AudioBriefingScriptFallback: settings.AudioBriefingScriptFallbackModel,
		TTSMarkupPreprocessModel:    settings.TTSMarkupPreprocessModel,
	}
}

func NewReadingPlanView(settings *model.UserSettings) ReadingPlanView {
	return ReadingPlanView{
		Window:          settings.ReadingPlanWindow,
		Size:            settings.ReadingPlanSize,
		DiversifyTopics: settings.ReadingPlanDiversifyTopics,
		ExcludeRead:     settings.ReadingPlanExcludeRead,
	}
}

func NewPodcastView(settings *model.UserSettings) PodcastView {
	var artworkURL *string
	if settings != nil {
		artworkURL = settings.PodcastArtworkURL
	}
	if artworkURL == nil {
		artworkURL = podcastDefaultArtworkURL()
	}
	if settings == nil {
		return PodcastView{
			Language:            "ja",
			AvailableCategories: PodcastCategoryDefinitions(),
			ArtworkURL:          artworkURL,
		}
	}
	return PodcastView{
		Enabled:             settings.PodcastEnabled,
		FeedSlug:            settings.PodcastFeedSlug,
		RSSURL:              podcastRSSURL(settings.PodcastFeedSlug),
		Title:               settings.PodcastTitle,
		Description:         settings.PodcastDescription,
		Author:              settings.PodcastAuthor,
		Language:            settings.PodcastLanguage,
		Category:            settings.PodcastCategory,
		Subcategory:         settings.PodcastSubcategory,
		AvailableCategories: PodcastCategoryDefinitions(),
		Explicit:            settings.PodcastExplicit,
		ArtworkURL:          artworkURL,
	}
}

func NewAudioBriefingView(settings *model.AudioBriefingSettings) AudioBriefingView {
	if settings == nil {
		return AudioBriefingView{
			ScheduleMode:                AudioBriefingScheduleModeInterval,
			IntervalHours:               6,
			ArticlesPerEpisode:          5,
			TargetDurationMinutes:       20,
			ChunkTrailingSilenceSeconds: 1.0,
			DefaultPersonaMode:          PersonaModeFixed,
			DefaultPersona:              "editor",
			ConversationMode:            "single",
		}
	}
	return AudioBriefingView{
		Enabled:                     settings.Enabled,
		ScheduleMode:                NormalizeAudioBriefingScheduleMode(settings.ScheduleMode),
		IntervalHours:               settings.IntervalHours,
		ArticlesPerEpisode:          settings.ArticlesPerEpisode,
		TargetDurationMinutes:       settings.TargetDurationMinutes,
		ChunkTrailingSilenceSeconds: settings.ChunkTrailingSilenceSeconds,
		ProgramName:                 settings.ProgramName,
		DefaultPersonaMode:          NormalizePersonaMode(&settings.DefaultPersonaMode),
		DefaultPersona:              settings.DefaultPersona,
		ConversationMode:            normalizeAudioBriefingConversationMode(&settings.ConversationMode),
		BGMEnabled:                  settings.BGMEnabled,
		BGMR2Prefix:                 settings.BGMR2Prefix,
	}
}

func NewAudioBriefingPersonaVoiceViews(rows []model.AudioBriefingPersonaVoice) []AudioBriefingPersonaVoiceView {
	out := make([]AudioBriefingPersonaVoiceView, 0, len(rows))
	for _, row := range rows {
		out = append(out, AudioBriefingPersonaVoiceView{
			Persona:                  row.Persona,
			TTSProvider:              row.TTSProvider,
			TTSModel:                 row.TTSModel,
			VoiceModel:               row.VoiceModel,
			VoiceStyle:               row.VoiceStyle,
			ProviderVoiceLabel:       row.ProviderVoiceLabel,
			ProviderVoiceDescription: row.ProviderVoiceDescription,
			SpeechRate:               row.SpeechRate,
			EmotionalIntensity:       row.EmotionalIntensity,
			TempoDynamics:            row.TempoDynamics,
			LineBreakSilenceSeconds:  row.LineBreakSilenceSeconds,
			Pitch:                    row.Pitch,
			VolumeGain:               row.VolumeGain,
		})
	}
	return out
}

func NewAudioBriefingPresetView(p model.AudioBriefingPreset) AudioBriefingPresetView {
	return AudioBriefingPresetView{
		ID:                 p.ID,
		Name:               p.Name,
		DefaultPersonaMode: NormalizePersonaMode(&p.DefaultPersonaMode),
		DefaultPersona:     p.DefaultPersona,
		ConversationMode:   normalizeAudioBriefingConversationMode(&p.ConversationMode),
		Voices:             NewAudioBriefingPersonaVoiceViews(p.Voices),
		CreatedAt:          p.CreatedAt,
		UpdatedAt:          p.UpdatedAt,
	}
}

func NewSummaryAudioView(settings *model.SummaryAudioVoiceSettings) SummaryAudioView {
	if settings == nil {
		return SummaryAudioView{}
	}
	return SummaryAudioView{
		TTSProvider:              settings.TTSProvider,
		TTSModel:                 settings.TTSModel,
		VoiceModel:               settings.VoiceModel,
		VoiceStyle:               settings.VoiceStyle,
		ProviderVoiceLabel:       settings.ProviderVoiceLabel,
		ProviderVoiceDescription: settings.ProviderVoiceDescription,
		SpeechRate:               settings.SpeechRate,
		EmotionalIntensity:       settings.EmotionalIntensity,
		TempoDynamics:            settings.TempoDynamics,
		LineBreakSilenceSeconds:  settings.LineBreakSilenceSeconds,
		Pitch:                    settings.Pitch,
		VolumeGain:               settings.VolumeGain,
		AivisUserDictionaryUUID:  settings.AivisUserDictionaryUUID,
	}
}

func NewObsidianExportView(settings *model.ObsidianExportSettings, githubApp *GitHubAppClient) ObsidianExportView {
	v := ObsidianExportView{
		Enabled:              settings.Enabled,
		GitHubInstallationID: settings.GitHubInstallationID,
		GitHubRepoOwner:      settings.GitHubRepoOwner,
		GitHubRepoName:       settings.GitHubRepoName,
		GitHubRepoBranch:     settings.GitHubRepoBranch,
		VaultRootPath:        settings.VaultRootPath,
		KeywordLinkMode:      settings.KeywordLinkMode,
		LastRunAt:            settings.LastRunAt,
		LastSuccessAt:        settings.LastSuccessAt,
	}
	if githubApp != nil {
		enabled := githubApp.Enabled()
		url := githubApp.InstallURL()
		v.GitHubAppEnabled = &enabled
		v.GitHubAppInstallURL = &url
	}
	return v
}

func NewNotificationPriorityView(rule *model.NotificationPriorityRule) NotificationPriorityView {
	if rule == nil {
		return NotificationPriorityView{
			Sensitivity:      "medium",
			DailyCap:         3,
			ThemeWeight:      1.0,
			ImmediateEnabled: true,
			BriefingEnabled:  true,
			ReviewEnabled:    true,
			GoalMatchEnabled: true,
		}
	}
	return NotificationPriorityView{
		ID:               rule.ID,
		Sensitivity:      rule.Sensitivity,
		DailyCap:         rule.DailyCap,
		ThemeWeight:      rule.ThemeWeight,
		ImmediateEnabled: rule.ImmediateEnabled,
		BriefingEnabled:  rule.BriefingEnabled,
		ReviewEnabled:    rule.ReviewEnabled,
		GoalMatchEnabled: rule.GoalMatchEnabled,
	}
}

func NewCurrentMonthView(monthStart, nextMonth time.Time, usedCostUSD float64, remainingBudgetUSD, remainingPct *float64) CurrentMonthView {
	return CurrentMonthView{
		MonthJST:           monthStart.Format("2006-01"),
		PeriodStartJST:     monthStart.Format(time.RFC3339),
		PeriodEndJST:       nextMonth.Format(time.RFC3339),
		EstimatedCostUSD:   usedCostUSD,
		RemainingBudgetUSD: remainingBudgetUSD,
		RemainingBudgetPct: remainingPct,
	}
}
