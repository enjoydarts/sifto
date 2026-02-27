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
	UserID                      string    `json:"user_id"`
	AnthropicAPIKeyLast4        *string   `json:"anthropic_api_key_last4,omitempty"`
	HasAnthropicAPIKey          bool      `json:"has_anthropic_api_key"`
	OpenAIAPIKeyLast4           *string   `json:"openai_api_key_last4,omitempty"`
	HasOpenAIAPIKey             bool      `json:"has_openai_api_key"`
	MonthlyBudgetUSD            *float64  `json:"monthly_budget_usd,omitempty"`
	BudgetAlertEnabled          bool      `json:"budget_alert_enabled"`
	BudgetAlertThresholdPct     int       `json:"budget_alert_threshold_pct"`
	DigestEmailEnabled          bool      `json:"digest_email_enabled"`
	ReadingPlanWindow           string    `json:"reading_plan_window"`
	ReadingPlanSize             int       `json:"reading_plan_size"`
	ReadingPlanDiversifyTopics  bool      `json:"reading_plan_diversify_topics"`
	ReadingPlanExcludeRead      bool      `json:"reading_plan_exclude_read"`
	AnthropicFactsModel         *string   `json:"anthropic_facts_model,omitempty"`
	AnthropicSummaryModel       *string   `json:"anthropic_summary_model,omitempty"`
	AnthropicDigestClusterModel *string   `json:"anthropic_digest_cluster_model,omitempty"`
	AnthropicDigestModel        *string   `json:"anthropic_digest_model,omitempty"`
	AnthropicSourceSuggestModel *string   `json:"anthropic_source_suggestion_model,omitempty"`
	OpenAIEmbeddingModel        *string   `json:"openai_embedding_model,omitempty"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
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

type Item struct {
	ID             string     `json:"id"`
	SourceID       string     `json:"source_id"`
	URL            string     `json:"url"`
	Title          *string    `json:"title"`
	ThumbnailURL   *string    `json:"thumbnail_url,omitempty"`
	ContentText    *string    `json:"content_text,omitempty"`
	Status         string     `json:"status"` // new | fetched | facts_extracted | summarized | failed
	IsRead         bool       `json:"is_read"`
	IsFavorite     bool       `json:"is_favorite"`
	FeedbackRating int        `json:"feedback_rating"` // -1 | 0 | 1
	SummaryScore   *float64   `json:"summary_score,omitempty"`
	SummaryTopics  []string   `json:"summary_topics,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	FetchedAt      *time.Time `json:"fetched_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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

type TopicTrend struct {
	Topic        string   `json:"topic"`
	Count24h     int      `json:"count_24h"`
	CountPrev24h int      `json:"count_prev_24h"`
	Delta        int      `json:"delta"`
	MaxScore24h  *float64 `json:"max_score_24h,omitempty"`
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
