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
	UserID                 string     `json:"user_id"`
	AnthropicAPIKeyLast4   *string    `json:"anthropic_api_key_last4,omitempty"`
	HasAnthropicAPIKey     bool       `json:"has_anthropic_api_key"`
	MonthlyBudgetUSD       *float64   `json:"monthly_budget_usd,omitempty"`
	BudgetAlertEnabled     bool       `json:"budget_alert_enabled"`
	BudgetAlertThresholdPct int       `json:"budget_alert_threshold_pct"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
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
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	URL         string     `json:"url"`
	Title       *string    `json:"title"`
	ContentText *string    `json:"content_text,omitempty"`
	Status      string     `json:"status"` // new | fetched | facts_extracted | summarized | failed
	SummaryScore *float64  `json:"summary_score,omitempty"`
	SummaryTopics []string `json:"summary_topics,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	FetchedAt   *time.Time `json:"fetched_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ItemFacts struct {
	ID          string    `json:"id"`
	ItemID      string    `json:"item_id"`
	Facts       []string  `json:"facts"`
	ExtractedAt time.Time `json:"extracted_at"`
}

type ItemSummary struct {
	ID           string    `json:"id"`
	ItemID       string    `json:"item_id"`
	Summary      string    `json:"summary"`
	Topics       []string  `json:"topics"`
	Score        *float64  `json:"score,omitempty"`
	ScoreBreakdown *ItemSummaryScoreBreakdown `json:"score_breakdown,omitempty"`
	ScoreReason  *string   `json:"score_reason,omitempty"`
	ScorePolicyVersion *string `json:"score_policy_version,omitempty"`
	SummarizedAt time.Time `json:"summarized_at"`
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
	Facts   *ItemFacts   `json:"facts,omitempty"`
	Summary *ItemSummary `json:"summary,omitempty"`
}

type Digest struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	DigestDate    string     `json:"digest_date"` // YYYY-MM-DD
	EmailSubject  *string    `json:"email_subject,omitempty"`
	EmailBody     *string    `json:"email_body,omitempty"`
	SendStatus    *string    `json:"send_status,omitempty"`
	SendError     *string    `json:"send_error,omitempty"`
	SendTriedAt   *time.Time `json:"send_tried_at,omitempty"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type DigestItem struct {
	ID       string `json:"id"`
	DigestID string `json:"digest_id"`
	ItemID   string `json:"item_id"`
	Rank     int    `json:"rank"`
}

type DigestDetail struct {
	Digest
	Items []DigestItemDetail `json:"items"`
}

type DigestItemDetail struct {
	Rank    int         `json:"rank"`
	Item    Item        `json:"item"`
	Summary ItemSummary `json:"summary"`
}
