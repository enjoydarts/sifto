package model

import "time"

type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Name            *string    `json:"name"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserIdentity struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	Email          *string   `json:"email,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserSettings struct {
	UserID                           string     `json:"user_id"`
	AnthropicAPIKeyLast4             *string    `json:"anthropic_api_key_last4,omitempty"`
	HasAnthropicAPIKey               bool       `json:"has_anthropic_api_key"`
	OpenAIAPIKeyLast4                *string    `json:"openai_api_key_last4,omitempty"`
	HasOpenAIAPIKey                  bool       `json:"has_openai_api_key"`
	GoogleAPIKeyLast4                *string    `json:"google_api_key_last4,omitempty"`
	HasGoogleAPIKey                  bool       `json:"has_google_api_key"`
	GroqAPIKeyLast4                  *string    `json:"groq_api_key_last4,omitempty"`
	HasGroqAPIKey                    bool       `json:"has_groq_api_key"`
	DeepSeekAPIKeyLast4              *string    `json:"deepseek_api_key_last4,omitempty"`
	HasDeepSeekAPIKey                bool       `json:"has_deepseek_api_key"`
	AlibabaAPIKeyLast4               *string    `json:"alibaba_api_key_last4,omitempty"`
	HasAlibabaAPIKey                 bool       `json:"has_alibaba_api_key"`
	MistralAPIKeyLast4               *string    `json:"mistral_api_key_last4,omitempty"`
	HasMistralAPIKey                 bool       `json:"has_mistral_api_key"`
	XAIAPIKeyLast4                   *string    `json:"xai_api_key_last4,omitempty"`
	HasXAIAPIKey                     bool       `json:"has_xai_api_key"`
	ZAIAPIKeyLast4                   *string    `json:"zai_api_key_last4,omitempty"`
	HasZAIAPIKey                     bool       `json:"has_zai_api_key"`
	FireworksAPIKeyLast4             *string    `json:"fireworks_api_key_last4,omitempty"`
	HasFireworksAPIKey               bool       `json:"has_fireworks_api_key"`
	PoeAPIKeyLast4                   *string    `json:"poe_api_key_last4,omitempty"`
	HasPoeAPIKey                     bool       `json:"has_poe_api_key"`
	OpenRouterAPIKeyLast4            *string    `json:"openrouter_api_key_last4,omitempty"`
	HasOpenRouterAPIKey              bool       `json:"has_openrouter_api_key"`
	AivisAPIKeyLast4                 *string    `json:"aivis_api_key_last4,omitempty"`
	HasAivisAPIKey                   bool       `json:"has_aivis_api_key"`
	MonthlyBudgetUSD                 *float64   `json:"monthly_budget_usd,omitempty"`
	BudgetAlertEnabled               bool       `json:"budget_alert_enabled"`
	BudgetAlertThresholdPct          int        `json:"budget_alert_threshold_pct"`
	DigestEmailEnabled               bool       `json:"digest_email_enabled"`
	ReadingPlanWindow                string     `json:"reading_plan_window"`
	ReadingPlanSize                  int        `json:"reading_plan_size"`
	ReadingPlanDiversifyTopics       bool       `json:"reading_plan_diversify_topics"`
	ReadingPlanExcludeRead           bool       `json:"reading_plan_exclude_read"`
	FactsModel                       *string    `json:"facts_model,omitempty"`
	FactsFallbackModel               *string    `json:"facts_fallback_model,omitempty"`
	SummaryModel                     *string    `json:"summary_model,omitempty"`
	SummaryFallbackModel             *string    `json:"summary_fallback_model,omitempty"`
	DigestClusterModel               *string    `json:"digest_cluster_model,omitempty"`
	DigestModel                      *string    `json:"digest_model,omitempty"`
	AskModel                         *string    `json:"ask_model,omitempty"`
	SourceSuggestionModel            *string    `json:"source_suggestion_model,omitempty"`
	EmbeddingModel                   *string    `json:"embedding_model,omitempty"`
	FactsCheckModel                  *string    `json:"facts_check_model,omitempty"`
	FaithfulnessCheckModel           *string    `json:"faithfulness_check_model,omitempty"`
	NavigatorEnabled                 bool       `json:"navigator_enabled"`
	NavigatorPersona                 string     `json:"navigator_persona"`
	NavigatorModel                   *string    `json:"navigator_model,omitempty"`
	NavigatorFallbackModel           *string    `json:"navigator_fallback_model,omitempty"`
	AudioBriefingScriptModel         *string    `json:"audio_briefing_script_model,omitempty"`
	AudioBriefingScriptFallbackModel *string    `json:"audio_briefing_script_fallback_model,omitempty"`
	HasInoreaderOAuth                bool       `json:"has_inoreader_oauth"`
	InoreaderTokenExpiresAt          *time.Time `json:"inoreader_token_expires_at,omitempty"`
	CreatedAt                        time.Time  `json:"created_at"`
	UpdatedAt                        time.Time  `json:"updated_at"`
}

type AudioBriefingSettings struct {
	UserID                string    `json:"user_id"`
	Enabled               bool      `json:"enabled"`
	IntervalHours         int       `json:"interval_hours"`
	ArticlesPerEpisode    int       `json:"articles_per_episode"`
	TargetDurationMinutes int       `json:"target_duration_minutes"`
	DefaultPersona        string    `json:"default_persona"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type AudioBriefingPersonaVoice struct {
	UserID                  string     `json:"user_id"`
	Persona                 string     `json:"persona"`
	TTSProvider             string     `json:"tts_provider"`
	VoiceModel              string     `json:"voice_model"`
	VoiceStyle              string     `json:"voice_style"`
	SpeechRate              float64    `json:"speech_rate"`
	EmotionalIntensity      float64    `json:"emotional_intensity"`
	TempoDynamics           float64    `json:"tempo_dynamics"`
	LineBreakSilenceSeconds float64    `json:"line_break_silence_seconds"`
	Pitch                   float64    `json:"pitch"`
	VolumeGain              float64    `json:"volume_gain"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	DeletedAt               *time.Time `json:"deleted_at,omitempty"`
}

type AudioBriefingJob struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	SlotStartedAtJST    time.Time  `json:"slot_started_at_jst"`
	SlotKey             string     `json:"slot_key"`
	Persona             string     `json:"persona"`
	Status              string     `json:"status"`
	SourceItemCount     int        `json:"source_item_count"`
	ReusedItemCount     int        `json:"reused_item_count"`
	ScriptCharCount     int        `json:"script_char_count"`
	AudioDurationSec    *int       `json:"audio_duration_sec,omitempty"`
	Title               *string    `json:"title,omitempty"`
	R2AudioObjectKey    *string    `json:"r2_audio_object_key,omitempty"`
	R2ManifestObjectKey *string    `json:"r2_manifest_object_key,omitempty"`
	ProviderJobID       *string    `json:"provider_job_id,omitempty"`
	IdempotencyKey      *string    `json:"idempotency_key,omitempty"`
	ErrorCode           *string    `json:"error_code,omitempty"`
	ErrorMessage        *string    `json:"error_message,omitempty"`
	PublishedAt         *time.Time `json:"published_at,omitempty"`
	FailedAt            *time.Time `json:"failed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type AudioBriefingJobItem struct {
	ID              string     `json:"id"`
	JobID           string     `json:"job_id"`
	ItemID          string     `json:"item_id"`
	Rank            int        `json:"rank"`
	SegmentTitle    *string    `json:"segment_title,omitempty"`
	SummarySnapshot *string    `json:"summary_snapshot,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	Title           *string    `json:"title,omitempty"`
	TranslatedTitle *string    `json:"translated_title,omitempty"`
	SourceTitle     *string    `json:"source_title,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
}

type AudioBriefingScriptChunk struct {
	ID               string    `json:"id"`
	JobID            string    `json:"job_id"`
	Seq              int       `json:"seq"`
	PartType         string    `json:"part_type"`
	Text             string    `json:"text"`
	CharCount        int       `json:"char_count"`
	TTSStatus        string    `json:"tts_status"`
	TTSProvider      *string   `json:"tts_provider,omitempty"`
	VoiceModel       *string   `json:"voice_model,omitempty"`
	VoiceStyle       *string   `json:"voice_style,omitempty"`
	R2AudioObjectKey *string   `json:"r2_audio_object_key,omitempty"`
	DurationSec      *int      `json:"duration_sec,omitempty"`
	ErrorMessage     *string   `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AudioBriefingCallbackToken struct {
	ID            string     `json:"id"`
	JobID         string     `json:"job_id"`
	RequestID     string     `json:"request_id"`
	ProviderJobID *string    `json:"provider_job_id,omitempty"`
	TokenHash     string     `json:"token_hash"`
	ExpiresAt     time.Time  `json:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ObsidianExportSettings struct {
	UserID               string     `json:"user_id"`
	Enabled              bool       `json:"enabled"`
	GitHubInstallationID *int64     `json:"github_installation_id,omitempty"`
	GitHubRepoOwner      *string    `json:"github_repo_owner,omitempty"`
	GitHubRepoName       *string    `json:"github_repo_name,omitempty"`
	GitHubRepoBranch     string     `json:"github_repo_branch"`
	VaultRootPath        *string    `json:"vault_root_path,omitempty"`
	KeywordLinkMode      string     `json:"keyword_link_mode"`
	LastRunAt            *time.Time `json:"last_run_at,omitempty"`
	LastSuccessAt        *time.Time `json:"last_success_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ItemExportRecord struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	ItemID      string     `json:"item_id"`
	Target      string     `json:"target"`
	GitHubPath  *string    `json:"github_path,omitempty"`
	GitHubSHA   *string    `json:"github_sha,omitempty"`
	ContentHash *string    `json:"content_hash,omitempty"`
	Status      string     `json:"status"`
	ExportedAt  *time.Time `json:"exported_at,omitempty"`
	LastError   *string    `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Source struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	URL           string     `json:"url"`
	Type          string     `json:"type"` // rss | manual
	Title         *string    `json:"title"`
	Enabled       bool       `json:"enabled"`
	LastFetchedAt *time.Time `json:"last_fetched_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type ReadingGoal struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type SourceHealth struct {
	SourceID      string     `json:"source_id"`
	TotalItems    int        `json:"total_items"`
	FailedItems   int        `json:"failed_items"`
	Summarized    int        `json:"summarized_items"`
	FailureRate   float64    `json:"failure_rate"`
	LastItemAt    *time.Time `json:"last_item_at,omitempty"`
	LastFetchedAt *time.Time `json:"last_fetched_at,omitempty"`
	Status        string     `json:"status"` // ok | stale | error | new | disabled
}

type SourceItemStats struct {
	SourceID             string  `json:"source_id"`
	TotalItems           int     `json:"total_items"`
	UnreadItems          int     `json:"unread_items"`
	ReadItems            int     `json:"read_items"`
	AvgItemsPerDay30Days float64 `json:"avg_items_per_day_30d"`
}

type SourceDailyCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type SourceDailyStats struct {
	SourceID               string             `json:"source_id"`
	TodayCount             int                `json:"today_count"`
	YesterdayCount         int                `json:"yesterday_count"`
	Last7DaysTotal         int                `json:"last_7d_total"`
	Last30DaysTotal        int                `json:"last_30d_total"`
	ActiveDays30d          int                `json:"active_days_30d"`
	AvgItemsPerActiveDay30 float64            `json:"avg_items_per_active_day_30d"`
	DailyCounts            []SourceDailyCount `json:"daily_counts"`
}

type SourceNavigatorCandidate struct {
	SourceID               string     `json:"source_id"`
	Title                  string     `json:"title"`
	URL                    string     `json:"url"`
	Enabled                bool       `json:"enabled"`
	Status                 string     `json:"status"`
	LastFetchedAt          *time.Time `json:"last_fetched_at,omitempty"`
	LastItemAt             *time.Time `json:"last_item_at,omitempty"`
	TotalItems30d          int        `json:"total_items_30d"`
	UnreadItems30d         int        `json:"unread_items_30d"`
	ReadItems30d           int        `json:"read_items_30d"`
	FavoriteCount30d       int        `json:"favorite_count_30d"`
	AvgItemsPerDay30d      float64    `json:"avg_items_per_day_30d"`
	ActiveDays30d          int        `json:"active_days_30d"`
	AvgItemsPerActiveDay30 float64    `json:"avg_items_per_active_day_30d"`
	FailureRate            float64    `json:"failure_rate"`
}

type SourceNavigatorPick struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Comment  string `json:"comment"`
}

type SourceNavigator struct {
	Enabled        bool                  `json:"enabled"`
	Persona        string                `json:"persona"`
	CharacterName  string                `json:"character_name"`
	CharacterTitle string                `json:"character_title"`
	AvatarStyle    string                `json:"avatar_style"`
	SpeechStyle    string                `json:"speech_style"`
	Overview       string                `json:"overview"`
	Keep           []SourceNavigatorPick `json:"keep"`
	Watch          []SourceNavigatorPick `json:"watch"`
	Standout       []SourceNavigatorPick `json:"standout"`
	GeneratedAt    *time.Time            `json:"generated_at,omitempty"`
}

type SourceNavigatorEnvelope struct {
	Navigator *SourceNavigator `json:"navigator,omitempty"`
}

type SourcesDailyOverview struct {
	TodayCount             int                `json:"today_count"`
	YesterdayCount         int                `json:"yesterday_count"`
	Last7DaysTotal         int                `json:"last_7d_total"`
	Last30DaysTotal        int                `json:"last_30d_total"`
	ActiveDays30d          int                `json:"active_days_30d"`
	AvgItemsPerActiveDay30 float64            `json:"avg_items_per_active_day_30d"`
	DailyCounts            []SourceDailyCount `json:"daily_counts"`
}

type RecommendedSource struct {
	SourceID         string     `json:"source_id"`
	URL              string     `json:"url"`
	Title            *string    `json:"title"`
	AffinityScore    float64    `json:"affinity_score"`
	ReadCount30d     int        `json:"read_count_30d"`
	Feedback30d      int        `json:"feedback_count_30d"`
	FavoriteCount30d int        `json:"favorite_count_30d"`
	LastItemAt       *time.Time `json:"last_item_at,omitempty"`
}

type Item struct {
	ID                     string                     `json:"id"`
	SourceID               string                     `json:"source_id"`
	SourceTitle            *string                    `json:"source_title,omitempty"`
	URL                    string                     `json:"url"`
	Title                  *string                    `json:"title"`
	ThumbnailURL           *string                    `json:"thumbnail_url,omitempty"`
	ContentText            *string                    `json:"content_text,omitempty"`
	Summary                *string                    `json:"summary,omitempty"`
	Status                 string                     `json:"status"` // new | fetched | facts_extracted | summarized | failed
	ProcessingError        *string                    `json:"processing_error,omitempty"`
	FactsCheckResult       *string                    `json:"facts_check_result,omitempty"`
	FaithfulnessResult     *string                    `json:"faithfulness_result,omitempty"`
	IsRead                 bool                       `json:"is_read"`
	IsFavorite             bool                       `json:"is_favorite"`
	FeedbackRating         int                        `json:"feedback_rating"` // -1 | 0 | 1
	SummaryScore           *float64                   `json:"summary_score,omitempty"`
	SummaryScoreBreakdown  *ItemSummaryScoreBreakdown `json:"summary_score_breakdown,omitempty"`
	PersonalScore          *float64                   `json:"personal_score,omitempty"`
	PersonalScoreReason    *string                    `json:"personal_score_reason,omitempty"`
	PersonalScoreBreakdown *PersonalScoreBreakdown    `json:"personal_score_breakdown,omitempty"`
	SummaryTopics          []string                   `json:"summary_topics,omitempty"`
	RecommendationReason   *string                    `json:"recommendation_reason,omitempty"`
	TranslatedTitle        *string                    `json:"translated_title,omitempty"`
	SearchMatchCount       int                        `json:"search_match_count,omitempty"`
	SearchSnippets         []ItemSearchSnippet        `json:"search_snippets,omitempty"`
	PublishedAt            *time.Time                 `json:"published_at,omitempty"`
	FetchedAt              *time.Time                 `json:"fetched_at,omitempty"`
	CreatedAt              time.Time                  `json:"created_at"`
	UpdatedAt              time.Time                  `json:"updated_at"`
}

type ItemSearchSnippet struct {
	Field       string `json:"field"`
	SnippetHTML string `json:"snippet_html"`
}

type ItemSearchDocument struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	SourceID        string     `json:"source_id"`
	Status          string     `json:"status"`
	IsDeleted       bool       `json:"is_deleted"`
	IsRead          bool       `json:"is_read"`
	IsFavorite      bool       `json:"is_favorite"`
	IsLater         bool       `json:"is_later"`
	Title           string     `json:"title"`
	TranslatedTitle string     `json:"translated_title"`
	Summary         string     `json:"summary"`
	FactsText       string     `json:"facts_text"`
	ContentText     string     `json:"content_text"`
	Topics          []string   `json:"topics"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type SearchSuggestionDocument struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Kind         string    `json:"kind"`
	Label        string    `json:"label"`
	Normalized   string    `json:"normalized"`
	Score        int       `json:"score"`
	ItemID       *string   `json:"item_id,omitempty"`
	SourceID     *string   `json:"source_id,omitempty"`
	Topic        *string   `json:"topic,omitempty"`
	ArticleCount *int      `json:"article_count,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SearchSuggestionItem struct {
	Kind         string  `json:"kind"`
	Label        string  `json:"label"`
	ItemID       *string `json:"item_id,omitempty"`
	SourceID     *string `json:"source_id,omitempty"`
	Topic        *string `json:"topic,omitempty"`
	ArticleCount *int    `json:"article_count,omitempty"`
}

type SearchSuggestionResponse struct {
	Items []SearchSuggestionItem `json:"items"`
}

type ItemFacts struct {
	ID          string    `json:"id"`
	ItemID      string    `json:"item_id"`
	Facts       []string  `json:"facts"`
	ExtractedAt time.Time `json:"extracted_at"`
}

type ItemSummary struct {
	ID                 string                     `json:"id"`
	ItemID             string                     `json:"item_id"`
	Summary            string                     `json:"summary"`
	Topics             []string                   `json:"topics"`
	TranslatedTitle    *string                    `json:"translated_title,omitempty"`
	Score              *float64                   `json:"score,omitempty"`
	ScoreBreakdown     *ItemSummaryScoreBreakdown `json:"score_breakdown,omitempty"`
	ScoreReason        *string                    `json:"score_reason,omitempty"`
	ScorePolicyVersion *string                    `json:"score_policy_version,omitempty"`
	SummarizedAt       time.Time                  `json:"summarized_at"`
}

type ItemSummaryLLM struct {
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	RequestedModel *string   `json:"requested_model,omitempty"`
	ResolvedModel  *string   `json:"resolved_model,omitempty"`
	PricingSource  string    `json:"pricing_source"`
	CreatedAt      time.Time `json:"created_at"`
}

type ItemLLMExecutionAttempt struct {
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Purpose      string    `json:"purpose"`
	Status       string    `json:"status"`
	AttemptIndex int       `json:"attempt_index"`
	ErrorKind    *string   `json:"error_kind,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type SummaryFaithfulnessCheck struct {
	ID           string    `json:"id"`
	ItemID       string    `json:"item_id"`
	FinalResult  string    `json:"final_result"`
	RetryCount   int       `json:"retry_count"`
	ShortComment *string   `json:"short_comment,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type FactsCheck struct {
	ID           string    `json:"id"`
	ItemID       string    `json:"item_id"`
	FinalResult  string    `json:"final_result"`
	RetryCount   int       `json:"retry_count"`
	ShortComment *string   `json:"short_comment,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ItemSummaryScoreBreakdown struct {
	Importance    *float64 `json:"importance,omitempty"`
	Novelty       *float64 `json:"novelty,omitempty"`
	Actionability *float64 `json:"actionability,omitempty"`
	Reliability   *float64 `json:"reliability,omitempty"`
	Relevance     *float64 `json:"relevance,omitempty"`
}

type PersonalScoreComponent struct {
	Value  float64 `json:"value"`
	Weight float64 `json:"weight"`
}

type PersonalScoreBreakdown struct {
	LearnedWeightScore  PersonalScoreComponent `json:"learned_weight_score"`
	TopicRelevance      PersonalScoreComponent `json:"topic_relevance"`
	EmbeddingSimilarity PersonalScoreComponent `json:"embedding_similarity"`
	SourceAffinity      PersonalScoreComponent `json:"source_affinity"`
	MatchedTopics       []string               `json:"matched_topics,omitempty"`
	DominantDimension   *string                `json:"dominant_dimension,omitempty"`
}

type ItemDetail struct {
	Item
	Facts             *ItemFacts                `json:"facts,omitempty"`
	FactsLLM          *ItemSummaryLLM           `json:"facts_llm,omitempty"`
	FactsExecutions   []ItemLLMExecutionAttempt `json:"facts_executions,omitempty"`
	FactsCheck        *FactsCheck               `json:"facts_check,omitempty"`
	FactsCheckLLM     *ItemSummaryLLM           `json:"facts_check_llm,omitempty"`
	Summary           *ItemSummary              `json:"summary,omitempty"`
	SummaryLLM        *ItemSummaryLLM           `json:"summary_llm,omitempty"`
	SummaryExecutions []ItemLLMExecutionAttempt `json:"summary_executions,omitempty"`
	Faithfulness      *SummaryFaithfulnessCheck `json:"faithfulness,omitempty"`
	FaithfulnessLLM   *ItemSummaryLLM           `json:"faithfulness_llm,omitempty"`
	Feedback          *ItemFeedback             `json:"feedback,omitempty"`
	Note              *ItemNote                 `json:"note,omitempty"`
	Highlights        []ItemHighlight           `json:"highlights,omitempty"`
}

type ItemFeedback struct {
	ItemID     string    `json:"item_id"`
	UserID     string    `json:"user_id"`
	Rating     int       `json:"rating"`      // -1 | 0 | 1
	IsFavorite bool      `json:"is_favorite"` // quick-save
	UpdatedAt  time.Time `json:"updated_at"`
}

type ItemNote struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ItemID    string    `json:"item_id"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ItemHighlight struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ItemID     string    `json:"item_id"`
	QuoteText  string    `json:"quote_text"`
	AnchorText string    `json:"anchor_text,omitempty"`
	Section    string    `json:"section,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type RelatedItem struct {
	ID           string     `json:"id"`
	SourceID     string     `json:"source_id"`
	URL          string     `json:"url"`
	Title        *string    `json:"title"`
	Summary      *string    `json:"summary,omitempty"`
	Topics       []string   `json:"topics,omitempty"`
	SummaryScore *float64   `json:"summary_score,omitempty"`
	Similarity   float64    `json:"similarity"`
	Reason       *string    `json:"reason,omitempty"`
	ReasonTopics []string   `json:"reason_topics,omitempty"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type AskCandidate struct {
	Item
	Summary    string   `json:"summary"`
	Facts      []string `json:"facts,omitempty"`
	Similarity float64  `json:"similarity"`
}

type AskCitation struct {
	ItemID      string   `json:"item_id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Reason      string   `json:"reason,omitempty"`
	PublishedAt *string  `json:"published_at,omitempty"`
	Topics      []string `json:"topics,omitempty"`
}

type AskResponse struct {
	Query        string         `json:"query"`
	Answer       string         `json:"answer"`
	Bullets      []string       `json:"bullets,omitempty"`
	Citations    []AskCitation  `json:"citations"`
	RelatedItems []AskCandidate `json:"related_items"`
	AskLLM       *AskLLM        `json:"ask_llm,omitempty"`
}

type AskNavigator struct {
	Enabled        bool       `json:"enabled"`
	Persona        string     `json:"persona"`
	CharacterName  string     `json:"character_name"`
	CharacterTitle string     `json:"character_title"`
	AvatarStyle    string     `json:"avatar_style"`
	SpeechStyle    string     `json:"speech_style"`
	Headline       string     `json:"headline"`
	Commentary     string     `json:"commentary"`
	NextAngles     []string   `json:"next_angles,omitempty"`
	GeneratedAt    *time.Time `json:"generated_at,omitempty"`
}

type AskNavigatorEnvelope struct {
	Navigator *AskNavigator `json:"navigator,omitempty"`
}

type AskLLM struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	PricingSource string `json:"pricing_source,omitempty"`
}

type ItemListResponse struct {
	Items             []Item  `json:"items"`
	Page              int     `json:"page"`
	PageSize          int     `json:"page_size"`
	Total             int     `json:"total"`
	HasNext           bool    `json:"has_next"`
	Sort              string  `json:"sort"`
	Status            *string `json:"status,omitempty"`
	SourceID          *string `json:"source_id,omitempty"`
	SearchMode        *string `json:"search_mode,omitempty"`
	SearchUnavailable bool    `json:"search_unavailable,omitempty"`
}

type FavoriteExportItem struct {
	ID              string          `json:"id"`
	URL             string          `json:"url"`
	Title           *string         `json:"title,omitempty"`
	TranslatedTitle *string         `json:"translated_title,omitempty"`
	SourceTitle     *string         `json:"source_title,omitempty"`
	Summary         *string         `json:"summary,omitempty"`
	Topics          []string        `json:"topics,omitempty"`
	SummaryScore    *float64        `json:"summary_score,omitempty"`
	PublishedAt     *time.Time      `json:"published_at,omitempty"`
	FavoritedAt     time.Time       `json:"favorited_at"`
	SummaryLLM      *ItemSummaryLLM `json:"summary_llm,omitempty"`
	FactsLLM        *ItemSummaryLLM `json:"facts_llm,omitempty"`
	EmbeddingModel  *string         `json:"embedding_model,omitempty"`
	Note            *ItemNote       `json:"note,omitempty"`
	Highlights      []ItemHighlight `json:"highlights,omitempty"`
}

type ReadingPlanResponse struct {
	Items           []Item               `json:"items"`
	Window          string               `json:"window"`
	Size            int                  `json:"size"`
	DiversifyTopics bool                 `json:"diversify_topics"`
	ExcludeRead     bool                 `json:"exclude_read"`
	SourcePoolCount int                  `json:"source_pool_count"`
	Topics          []ReadingPlanTopic   `json:"topics"`
	Clusters        []ReadingPlanCluster `json:"clusters,omitempty"`
}

type ReadingPlanTopic struct {
	Topic    string   `json:"topic"`
	Count    int      `json:"count"`
	MaxScore *float64 `json:"max_score,omitempty"`
}

type ReadingPlanCluster struct {
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	Size           int     `json:"size"`
	MaxSimilarity  float64 `json:"max_similarity"`
	Representative Item    `json:"representative"`
	Items          []Item  `json:"items"`
}

type TriageBundle struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Size           int      `json:"size"`
	MaxSimilarity  float64  `json:"max_similarity"`
	Representative Item     `json:"representative"`
	Items          []Item   `json:"items"`
	Summary        *string  `json:"summary,omitempty"`
	SharedTopics   []string `json:"shared_topics,omitempty"`
}

type TriageQueueEntry struct {
	EntryType string        `json:"entry_type"`
	Item      *Item         `json:"item,omitempty"`
	Bundle    *TriageBundle `json:"bundle,omitempty"`
}

type TriageQueueResponse struct {
	Entries         []TriageQueueEntry `json:"entries"`
	Window          string             `json:"window"`
	Size            int                `json:"size"`
	Completed       int                `json:"completed"`
	Remaining       int                `json:"remaining"`
	Total           int                `json:"total"`
	UnderlyingItems int                `json:"underlying_items"`
	BundleCount     int                `json:"bundle_count"`
	SourcePool      int                `json:"source_pool"`
	DiversifyTopics bool               `json:"diversify_topics"`
}

type TodayQueueCandidate struct {
	Item          Item       `json:"item"`
	LastSkippedAt *time.Time `json:"last_skipped_at,omitempty"`
}

type TodayQueueItem struct {
	Item                    Item          `json:"item"`
	EstimatedReadingMinutes int           `json:"estimated_reading_minutes"`
	ReasonLabels            []string      `json:"reason_labels"`
	MatchedGoals            []ReadingGoal `json:"matched_goals,omitempty"`
}

type TodayQueueResponse struct {
	Items []TodayQueueItem `json:"items"`
}

type ReviewQueueItem struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	ItemID         string     `json:"item_id"`
	SourceSignal   string     `json:"source_signal"`
	ReviewStage    string     `json:"review_stage"`
	Status         string     `json:"status"`
	ReviewDueAt    time.Time  `json:"review_due_at"`
	LastSurfacedAt *time.Time `json:"last_surfaced_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	SnoozeCount    int        `json:"snooze_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Item           Item       `json:"item"`
	ReasonLabels   []string   `json:"reason_labels,omitempty"`
}

type ReviewQueueResponse struct {
	Items []ReviewQueueItem `json:"items"`
}

type AskInsightItemRef struct {
	ItemID string   `json:"item_id"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Topics []string `json:"topics,omitempty"`
}

type AskInsight struct {
	ID        string              `json:"id"`
	UserID    string              `json:"user_id"`
	Title     string              `json:"title"`
	Body      string              `json:"body"`
	Query     string              `json:"query,omitempty"`
	GoalID    *string             `json:"goal_id,omitempty"`
	Tags      []string            `json:"tags,omitempty"`
	Items     []AskInsightItemRef `json:"items,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type WeeklyReviewTopic struct {
	Topic string `json:"topic"`
	Count int    `json:"count"`
}

type WeeklyReviewSnapshot struct {
	ID              string              `json:"id"`
	UserID          string              `json:"user_id"`
	WeekStart       string              `json:"week_start"`
	WeekEnd         string              `json:"week_end"`
	ReadCount       int                 `json:"read_count"`
	NoteCount       int                 `json:"note_count"`
	InsightCount    int                 `json:"insight_count"`
	FavoriteCount   int                 `json:"favorite_count"`
	DominantTopics  []WeeklyReviewTopic `json:"dominant_topics,omitempty"`
	MissedHighValue []Item              `json:"missed_high_value,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
}

type SourceOptimizationSnapshot struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	SourceID       string    `json:"source_id"`
	WindowStart    string    `json:"window_start"`
	WindowEnd      string    `json:"window_end"`
	Metrics        any       `json:"metrics"`
	Recommendation string    `json:"recommendation"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type NotificationPriorityRule struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Sensitivity      string    `json:"sensitivity"`
	DailyCap         int       `json:"daily_cap"`
	ThemeWeight      float64   `json:"theme_weight"`
	ImmediateEnabled bool      `json:"immediate_enabled"`
	BriefingEnabled  bool      `json:"briefing_enabled"`
	ReviewEnabled    bool      `json:"review_enabled"`
	GoalMatchEnabled bool      `json:"goal_match_enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ItemStatsResponse struct {
	Total    int            `json:"total"`
	Read     int            `json:"read"`
	Unread   int            `json:"unread"`
	ByStatus map[string]int `json:"by_status"`
}

type ItemUXMetricsResponse struct {
	Days                     int      `json:"days"`
	TodayDate                string   `json:"today_date"`
	TodayNewItems            int      `json:"today_new_items"`
	TodayReadItems           int      `json:"today_read_items"`
	TodayConsumptionRate     *float64 `json:"today_consumption_rate,omitempty"`
	PeriodReadItems          int      `json:"period_read_items"`
	PeriodActiveReadDays     int      `json:"period_active_read_days"`
	PeriodAverageReadsPerDay float64  `json:"period_average_reads_per_day"`
	CurrentStreakDays        int      `json:"current_streak_days"`
}

type TopicTrend struct {
	Topic        string   `json:"topic"`
	Count24h     int      `json:"count_24h"`
	CountPrev24h int      `json:"count_prev_24h"`
	Delta        int      `json:"delta"`
	MaxScore24h  *float64 `json:"max_score_24h,omitempty"`
}

type TopicPulsePoint struct {
	Date     string   `json:"date"`
	Count    int      `json:"count"`
	MaxScore *float64 `json:"max_score,omitempty"`
}

type TopicPulseItem struct {
	Topic    string            `json:"topic"`
	Total    int               `json:"total"`
	Delta    int               `json:"delta"`
	MaxScore *float64          `json:"max_score,omitempty"`
	Points   []TopicPulsePoint `json:"points"`
}

type BriefingCluster struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Summary  string   `json:"summary,omitempty"`
	MaxScore *float64 `json:"max_score,omitempty"`
	Topics   []string `json:"topics,omitempty"`
	Items    []Item   `json:"items,omitempty"`
}

type BriefingStats struct {
	TotalUnread         int  `json:"total_unread"`
	TodayHighlightCount int  `json:"today_highlight_count"`
	YesterdayRead       int  `json:"yesterday_read"`
	YesterdaySkipped    int  `json:"yesterday_skipped"`
	StreakDays          int  `json:"streak_days"`
	TodayReadCount      int  `json:"today_read_count"`
	StreakTarget        int  `json:"streak_target"`
	StreakRemaining     int  `json:"streak_remaining"`
	StreakAtRisk        bool `json:"streak_at_risk"`
}

type BriefingTodayResponse struct {
	Date           string             `json:"date"`
	Greeting       string             `json:"greeting"`
	GreetingKey    string             `json:"greeting_key,omitempty"`
	Status         string             `json:"status"` // pending | ready | stale
	GeneratedAt    *time.Time         `json:"generated_at,omitempty"`
	HighlightItems []Item             `json:"highlight_items"`
	Clusters       []BriefingCluster  `json:"clusters"`
	Stats          BriefingStats      `json:"stats"`
	Navigator      *BriefingNavigator `json:"navigator,omitempty"`
}

type BriefingNavigatorPick struct {
	ItemID      string   `json:"item_id"`
	Rank        int      `json:"rank"`
	Title       string   `json:"title"`
	SourceTitle *string  `json:"source_title,omitempty"`
	Comment     string   `json:"comment"`
	ReasonTags  []string `json:"reason_tags,omitempty"`
}

type BriefingNavigatorCandidate struct {
	ItemID          string     `json:"item_id"`
	Title           *string    `json:"title,omitempty"`
	TranslatedTitle *string    `json:"translated_title,omitempty"`
	SourceTitle     *string    `json:"source_title,omitempty"`
	Summary         string     `json:"summary"`
	Topics          []string   `json:"topics,omitempty"`
	Score           *float64   `json:"score,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
}

type BriefingNavigator struct {
	Enabled        bool                    `json:"enabled"`
	Persona        string                  `json:"persona"`
	CharacterName  string                  `json:"character_name"`
	CharacterTitle string                  `json:"character_title"`
	AvatarStyle    string                  `json:"avatar_style"`
	SpeechStyle    string                  `json:"speech_style"`
	Intro          string                  `json:"intro"`
	GeneratedAt    *time.Time              `json:"generated_at,omitempty"`
	Picks          []BriefingNavigatorPick `json:"picks"`
}

type BriefingNavigatorEnvelope struct {
	Navigator *BriefingNavigator `json:"navigator,omitempty"`
}

type ItemNavigator struct {
	Enabled        bool       `json:"enabled"`
	ItemID         string     `json:"item_id"`
	Persona        string     `json:"persona"`
	CharacterName  string     `json:"character_name"`
	CharacterTitle string     `json:"character_title"`
	AvatarStyle    string     `json:"avatar_style"`
	SpeechStyle    string     `json:"speech_style"`
	Headline       string     `json:"headline"`
	Commentary     string     `json:"commentary"`
	StanceTags     []string   `json:"stance_tags,omitempty"`
	GeneratedAt    *time.Time `json:"generated_at,omitempty"`
}

type ItemNavigatorEnvelope struct {
	Navigator *ItemNavigator `json:"navigator,omitempty"`
}

type Digest struct {
	ID                     string     `json:"id"`
	UserID                 string     `json:"user_id"`
	DigestDate             string     `json:"digest_date"` // YYYY-MM-DD
	EmailSubject           *string    `json:"email_subject,omitempty"`
	EmailBody              *string    `json:"email_body,omitempty"`
	DigestRetryCount       int        `json:"digest_retry_count"`
	ClusterDraftRetryCount int        `json:"cluster_draft_retry_count"`
	SendStatus             *string    `json:"send_status,omitempty"`
	SendError              *string    `json:"send_error,omitempty"`
	SendTriedAt            *time.Time `json:"send_tried_at,omitempty"`
	SentAt                 *time.Time `json:"sent_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
}

type DigestItem struct {
	ID       string `json:"id"`
	DigestID string `json:"digest_id"`
	ItemID   string `json:"item_id"`
	Rank     int    `json:"rank"`
}

type DigestDetail struct {
	Digest
	DigestLLM       *ItemSummaryLLM      `json:"digest_llm,omitempty"`
	ClusterDraftLLM *ItemSummaryLLM      `json:"cluster_draft_llm,omitempty"`
	Items           []DigestItemDetail   `json:"items"`
	ClusterDrafts   []DigestClusterDraft `json:"cluster_drafts,omitempty"`
}

type DigestItemDetail struct {
	Rank    int         `json:"rank"`
	Item    Item        `json:"item"`
	Summary ItemSummary `json:"summary"`
	Facts   []string    `json:"facts,omitempty"`
}

type DigestClusterDraft struct {
	ID           string    `json:"id"`
	DigestID     string    `json:"digest_id"`
	ClusterKey   string    `json:"cluster_key"`
	ClusterLabel string    `json:"cluster_label"`
	Rank         int       `json:"rank"`
	ItemCount    int       `json:"item_count"`
	Topics       []string  `json:"topics"`
	MaxScore     *float64  `json:"max_score,omitempty"`
	DraftSummary string    `json:"draft_summary"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserPreferenceProfile struct {
	UserID           string             `json:"user_id"`
	LearnedWeights   map[string]float64 `json:"learned_weights"`
	TopicInterests   map[string]float64 `json:"topic_interests"`
	PrefEmbedding    []float64          `json:"pref_embedding,omitempty"`
	SourceAffinities map[string]float64 `json:"source_affinities"`
	FeedbackCount    int                `json:"feedback_count"`
	ReadCount        int                `json:"read_count"`
	ComputedAt       *time.Time         `json:"computed_at,omitempty"`
}

type PreferenceProfileWeight struct {
	Value   float64 `json:"value"`
	Default float64 `json:"default"`
	Delta   float64 `json:"delta"`
}

type PreferenceProfileTopic struct {
	Topic       string  `json:"topic"`
	Score       float64 `json:"score"`
	SignalCount int     `json:"signal_count"`
}

type PreferenceProfileSource struct {
	SourceID    string  `json:"source_id"`
	SourceTitle string  `json:"source_title"`
	Score       float64 `json:"score"`
}

type PreferenceProfileReadingPattern struct {
	AvgScoreRead    float64 `json:"avg_score_read"`
	AvgScoreSkipped float64 `json:"avg_score_skipped"`
	FavoriteRate    float64 `json:"favorite_rate"`
	NoteRate        float64 `json:"note_rate"`
}

type PreferenceProfileResponse struct {
	Status         string                             `json:"status"`
	Confidence     float64                            `json:"confidence"`
	FeedbackCount  int                                `json:"feedback_count"`
	ReadCount      int                                `json:"read_count"`
	ComputedAt     *time.Time                         `json:"computed_at,omitempty"`
	LearnedWeights map[string]PreferenceProfileWeight `json:"learned_weights"`
	TopTopics      []PreferenceProfileTopic           `json:"top_topics"`
	TopSources     []PreferenceProfileSource          `json:"top_sources"`
	ReadingPattern PreferenceProfileReadingPattern    `json:"reading_pattern"`
}

type PreferenceProfileSummaryResponse struct {
	Status          string     `json:"status"`
	Confidence      float64    `json:"confidence"`
	FeedbackCount   int        `json:"feedback_count"`
	TopTopics       []string   `json:"top_topics"`
	StrongestWeight string     `json:"strongest_weight"`
	ComputedAt      *time.Time `json:"computed_at,omitempty"`
}

var DefaultScoreWeights = map[string]float64{
	"importance":    0.38,
	"novelty":       0.22,
	"actionability": 0.18,
	"reliability":   0.17,
	"relevance":     0.05,
}
