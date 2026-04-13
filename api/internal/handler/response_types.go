package handler

import (
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type itemToggleResponse struct {
	ItemID string `json:"item_id"`
	IsRead bool   `json:"is_read"`
}

type itemLaterResponse struct {
	ItemID  string `json:"item_id"`
	IsLater bool   `json:"is_later"`
}

type bulkStatusResponse struct {
	Status       string `json:"status"`
	UpdatedCount int    `json:"updated_count"`
}

type topicTrendsResponse struct {
	Items []model.TopicTrend `json:"items"`
	Limit int                `json:"limit"`
}

type topicPulseResponse struct {
	Days  int                    `json:"days"`
	Limit int                    `json:"limit"`
	Items []model.TopicPulseItem `json:"items"`
}

type focusQueueResponse struct {
	Items           []model.Item `json:"items"`
	Size            int          `json:"size"`
	Window          string       `json:"window"`
	Completed       int          `json:"completed"`
	Remaining       int          `json:"remaining"`
	Total           int          `json:"total"`
	SourcePool      int          `json:"source_pool"`
	DiversifyTopics bool         `json:"diversify_topics"`
}

type relatedItemsResponse struct {
	Items    []model.RelatedItem      `json:"items"`
	Clusters []relatedClusterResponse `json:"clusters"`
	Limit    int                      `json:"limit"`
	ItemID   string                   `json:"item_id"`
}

type retryItemResponse struct {
	Status string `json:"status"`
	ItemID string `json:"item_id"`
}

type retryFailedResponse struct {
	Status      string  `json:"status"`
	SourceID    *string `json:"source_id"`
	Matched     int     `json:"matched"`
	QueuedCount int     `json:"queued_count"`
	FailedCount int     `json:"failed_count"`
}

type sourceListItemsResponse struct {
	Items any `json:"items"`
}

type sourceDailyStatsResponse struct {
	Items    []model.SourceDailyStats   `json:"items"`
	Overview model.SourcesDailyOverview `json:"overview"`
}

type sourceRecommendResponse struct {
	Items []service.SourceSuggestionResponse `json:"items"`
	Limit int                                `json:"limit"`
	LLM   any                                `json:"llm"`
}

type discoverFeedsResponse struct {
	Feeds []service.FeedCandidate `json:"feeds"`
}

type dashboardResponse struct {
	SourcesCount       any `json:"sources_count"`
	ItemStats          any `json:"item_stats"`
	Digests            any `json:"digests"`
	LLMSummary         any `json:"llm_summary"`
	TopicTrends        any `json:"topic_trends"`
	FailedItemsPreview any `json:"failed_items_preview"`
	LLMDays            int `json:"llm_days"`
}

type dashboardTopicTrends struct {
	Items  any    `json:"items"`
	Limit  int    `json:"limit"`
	Period string `json:"period"`
}

type importResultResponse struct {
	Status      string   `json:"status"`
	Total       int      `json:"total"`
	Added       int      `json:"added"`
	Skipped     int      `json:"skipped"`
	Invalid     int      `json:"invalid"`
	ErrorCount  int      `json:"error_count"`
	ErrorSample []string `json:"error_sample"`
}
