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

type UserSettings struct {
	UserID                      string     `json:"user_id"`
	AnthropicAPIKeyLast4        *string    `json:"anthropic_api_key_last4,omitempty"`
	HasAnthropicAPIKey          bool       `json:"has_anthropic_api_key"`
	OpenAIAPIKeyLast4           *string    `json:"openai_api_key_last4,omitempty"`
	HasOpenAIAPIKey             bool       `json:"has_openai_api_key"`
	GoogleAPIKeyLast4           *string    `json:"google_api_key_last4,omitempty"`
	HasGoogleAPIKey             bool       `json:"has_google_api_key"`
	MonthlyBudgetUSD            *float64   `json:"monthly_budget_usd,omitempty"`
	BudgetAlertEnabled          bool       `json:"budget_alert_enabled"`
	BudgetAlertThresholdPct     int        `json:"budget_alert_threshold_pct"`
	DigestEmailEnabled          bool       `json:"digest_email_enabled"`
	ReadingPlanWindow           string     `json:"reading_plan_window"`
	ReadingPlanSize             int        `json:"reading_plan_size"`
	ReadingPlanDiversifyTopics  bool       `json:"reading_plan_diversify_topics"`
	ReadingPlanExcludeRead      bool       `json:"reading_plan_exclude_read"`
	AnthropicFactsModel         *string    `json:"anthropic_facts_model,omitempty"`
	AnthropicSummaryModel       *string    `json:"anthropic_summary_model,omitempty"`
	AnthropicDigestClusterModel *string    `json:"anthropic_digest_cluster_model,omitempty"`
	AnthropicDigestModel        *string    `json:"anthropic_digest_model,omitempty"`
	AnthropicSourceSuggestModel *string    `json:"anthropic_source_suggestion_model,omitempty"`
	OpenAIEmbeddingModel        *string    `json:"openai_embedding_model,omitempty"`
	HasInoreaderOAuth           bool       `json:"has_inoreader_oauth"`
	InoreaderTokenExpiresAt     *time.Time `json:"inoreader_token_expires_at,omitempty"`
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
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
	ID              string     `json:"id"`
	SourceID        string     `json:"source_id"`
	URL             string     `json:"url"`
	Title           *string    `json:"title"`
	ThumbnailURL    *string    `json:"thumbnail_url,omitempty"`
	ContentText     *string    `json:"content_text,omitempty"`
	Status          string     `json:"status"` // new | fetched | facts_extracted | summarized | failed
	IsRead          bool       `json:"is_read"`
	IsFavorite      bool       `json:"is_favorite"`
	FeedbackRating  int        `json:"feedback_rating"` // -1 | 0 | 1
	SummaryScore    *float64   `json:"summary_score,omitempty"`
	SummaryTopics   []string   `json:"summary_topics,omitempty"`
	TranslatedTitle *string    `json:"translated_title,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	FetchedAt       *time.Time `json:"fetched_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	PricingSource string    `json:"pricing_source"`
	CreatedAt     time.Time `json:"created_at"`
}

type ItemSummaryScoreBreakdown struct {
	Importance    *float64 `json:"importance,omitempty"`
	Novelty       *float64 `json:"novelty,omitempty"`
	Actionability *float64 `json:"actionability,omitempty"`
	Reliability   *float64 `json:"reliability,omitempty"`
	Relevance     *float64 `json:"relevance,omitempty"`
}

type ItemDetail struct {
	Item
	ProcessingError *string         `json:"processing_error,omitempty"`
	Facts           *ItemFacts      `json:"facts,omitempty"`
	Summary         *ItemSummary    `json:"summary,omitempty"`
	SummaryLLM      *ItemSummaryLLM `json:"summary_llm,omitempty"`
	Feedback        *ItemFeedback   `json:"feedback,omitempty"`
}

type ItemFeedback struct {
	ItemID     string    `json:"item_id"`
	UserID     string    `json:"user_id"`
	Rating     int       `json:"rating"`      // -1 | 0 | 1
	IsFavorite bool      `json:"is_favorite"` // quick-save
	UpdatedAt  time.Time `json:"updated_at"`
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

type ItemListResponse struct {
	Items    []Item  `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Total    int     `json:"total"`
	HasNext  bool    `json:"has_next"`
	Sort     string  `json:"sort"`
	Status   *string `json:"status,omitempty"`
	SourceID *string `json:"source_id,omitempty"`
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
	Date           string            `json:"date"`
	Greeting       string            `json:"greeting"`
	GreetingKey    string            `json:"greeting_key,omitempty"`
	Status         string            `json:"status"` // pending | ready | stale
	GeneratedAt    *time.Time        `json:"generated_at,omitempty"`
	HighlightItems []Item            `json:"highlight_items"`
	Clusters       []BriefingCluster `json:"clusters"`
	Stats          BriefingStats     `json:"stats"`
}

type Digest struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	DigestDate   string     `json:"digest_date"` // YYYY-MM-DD
	EmailSubject *string    `json:"email_subject,omitempty"`
	EmailBody    *string    `json:"email_body,omitempty"`
	SendStatus   *string    `json:"send_status,omitempty"`
	SendError    *string    `json:"send_error,omitempty"`
	SendTriedAt  *time.Time `json:"send_tried_at,omitempty"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type DigestItem struct {
	ID       string `json:"id"`
	DigestID string `json:"digest_id"`
	ItemID   string `json:"item_id"`
	Rank     int    `json:"rank"`
}

type DigestDetail struct {
	Digest
	Items         []DigestItemDetail   `json:"items"`
	ClusterDrafts []DigestClusterDraft `json:"cluster_drafts,omitempty"`
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
