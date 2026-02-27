package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type WorkerClient struct {
	baseURL              string
	http                 *http.Client
	composeDigestTimeout time.Duration
	internalSecret       string
}

func NewWorkerClient() *WorkerClient {
	url := os.Getenv("PYTHON_WORKER_URL")
	if url == "" {
		url = "http://localhost:8000"
	}
	return &WorkerClient{
		baseURL:              url,
		http:                 &http.Client{Timeout: 60 * time.Second},
		composeDigestTimeout: workerComposeDigestTimeout(),
		internalSecret:       strings.TrimSpace(os.Getenv("INTERNAL_WORKER_SECRET")),
	}
}

func workerComposeDigestTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("PYTHON_WORKER_COMPOSE_DIGEST_TIMEOUT_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 180 * time.Second
}

type ExtractBodyResponse struct {
	Title       *string `json:"title"`
	Content     string  `json:"content"`
	PublishedAt *string `json:"published_at"`
	ImageURL    *string `json:"image_url"`
}

type ExtractFactsResponse struct {
	Facts []string  `json:"facts"`
	LLM   *LLMUsage `json:"llm,omitempty"`
}

type SummarizeResponse struct {
	Summary            string         `json:"summary"`
	Topics             []string       `json:"topics"`
	Score              float64        `json:"score"`
	ScoreBreakdown     map[string]any `json:"score_breakdown,omitempty"`
	ScoreReason        string         `json:"score_reason,omitempty"`
	ScorePolicyVersion string         `json:"score_policy_version,omitempty"`
	LLM                *LLMUsage      `json:"llm,omitempty"`
}

type ComposeDigestItem struct {
	Rank    int      `json:"rank"`
	Title   *string  `json:"title"`
	URL     string   `json:"url"`
	Summary string   `json:"summary"`
	Topics  []string `json:"topics"`
	Score   *float64 `json:"score,omitempty"`
}

type ComposeDigestResponse struct {
	Subject string    `json:"subject"`
	Body    string    `json:"body"`
	LLM     *LLMUsage `json:"llm,omitempty"`
}

type ComposeDigestClusterDraftResponse struct {
	DraftSummary string    `json:"draft_summary"`
	LLM          *LLMUsage `json:"llm,omitempty"`
}

type RankFeedSuggestionsCandidate struct {
	URL           string   `json:"url"`
	Title         *string  `json:"title,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
	MatchedTopics []string `json:"matched_topics,omitempty"`
}

type RankFeedSuggestionsExistingSource struct {
	URL   string  `json:"url"`
	Title *string `json:"title,omitempty"`
}

type RankFeedSuggestionsItem struct {
	URL        string  `json:"url"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

type RankFeedSuggestionsResponse struct {
	Items []RankFeedSuggestionsItem `json:"items"`
	LLM   *LLMUsage                 `json:"llm,omitempty"`
}

type SuggestFeedSeedSitesItem struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

type SuggestFeedSeedSitesResponse struct {
	Items []SuggestFeedSeedSitesItem `json:"items"`
	LLM   *LLMUsage                  `json:"llm,omitempty"`
}

type LLMUsage struct {
	Provider                 string  `json:"provider"`
	Model                    string  `json:"model"`
	PricingModelFamily       string  `json:"pricing_model_family,omitempty"`
	PricingSource            string  `json:"pricing_source,omitempty"`
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

func (w *WorkerClient) ExtractBody(ctx context.Context, url string) (*ExtractBodyResponse, error) {
	return postWithHeaders[ExtractBodyResponse](ctx, w, "/extract-body", map[string]any{"url": url}, workerHeaders(nil, nil, w.internalSecret))
}

func (w *WorkerClient) Health(ctx context.Context) error {
	if w == nil {
		return fmt.Errorf("worker client is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(b) > 0 {
			return fmt.Errorf("worker /health: status %d body=%s", resp.StatusCode, string(b))
		}
		return fmt.Errorf("worker /health: status %d", resp.StatusCode)
	}
	return nil
}

func (w *WorkerClient) ExtractFacts(ctx context.Context, title *string, content string, anthropicAPIKey *string, googleAPIKey *string) (*ExtractFactsResponse, error) {
	return postWithHeaders[ExtractFactsResponse](ctx, w, "/extract-facts", map[string]any{
		"title":   title,
		"content": content,
		"model":   nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) ExtractFactsWithModel(ctx context.Context, title *string, content string, anthropicAPIKey *string, googleAPIKey *string, model *string) (*ExtractFactsResponse, error) {
	return postWithHeaders[ExtractFactsResponse](ctx, w, "/extract-facts", map[string]any{
		"title":   title,
		"content": content,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) Summarize(ctx context.Context, title *string, facts []string, anthropicAPIKey *string, googleAPIKey *string) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title":             title,
		"facts":             facts,
		"model":             nil,
		"source_text_chars": nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) SummarizeWithModel(ctx context.Context, title *string, facts []string, sourceTextChars *int, anthropicAPIKey *string, googleAPIKey *string, model *string) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title":             title,
		"facts":             facts,
		"model":             model,
		"source_text_chars": sourceTextChars,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) ComposeDigest(ctx context.Context, digestDate string, items []ComposeDigestItem, anthropicAPIKey *string, googleAPIKey *string) (*ComposeDigestResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.composeDigestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.composeDigestTimeout)
		defer cancel()
	}
	return postWithHeaders[ComposeDigestResponse](ctx, w, "/compose-digest", map[string]any{
		"digest_date": digestDate,
		"items":       items,
		"model":       nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) ComposeDigestWithModel(ctx context.Context, digestDate string, items []ComposeDigestItem, anthropicAPIKey *string, googleAPIKey *string, model *string) (*ComposeDigestResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.composeDigestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.composeDigestTimeout)
		defer cancel()
	}
	return postWithHeaders[ComposeDigestResponse](ctx, w, "/compose-digest", map[string]any{
		"digest_date": digestDate,
		"items":       items,
		"model":       model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) ComposeDigestClusterDraftWithModel(
	ctx context.Context,
	clusterLabel string,
	itemCount int,
	topics []string,
	sourceLines []string,
	anthropicAPIKey *string,
	googleAPIKey *string,
	model *string,
) (*ComposeDigestClusterDraftResponse, error) {
	return postWithHeaders[ComposeDigestClusterDraftResponse](ctx, w, "/compose-digest-cluster-draft", map[string]any{
		"cluster_label": clusterLabel,
		"item_count":    itemCount,
		"topics":        topics,
		"source_lines":  sourceLines,
		"model":         model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, w.internalSecret))
}

func (w *WorkerClient) RankFeedSuggestions(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	candidates []RankFeedSuggestionsCandidate,
	anthropicAPIKey *string,
) (*RankFeedSuggestionsResponse, error) {
	return postWithHeaders[RankFeedSuggestionsResponse](ctx, w, "/rank-feed-suggestions", map[string]any{
		"existing_sources": existing,
		"preferred_topics": preferredTopics,
		"candidates":       candidates,
		"model":            nil,
	}, workerHeaders(anthropicAPIKey, nil, w.internalSecret))
}

func (w *WorkerClient) RankFeedSuggestionsWithModel(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	candidates []RankFeedSuggestionsCandidate,
	anthropicAPIKey *string,
	model *string,
) (*RankFeedSuggestionsResponse, error) {
	return postWithHeaders[RankFeedSuggestionsResponse](ctx, w, "/rank-feed-suggestions", map[string]any{
		"existing_sources": existing,
		"preferred_topics": preferredTopics,
		"candidates":       candidates,
		"model":            model,
	}, workerHeaders(anthropicAPIKey, nil, w.internalSecret))
}

func (w *WorkerClient) SuggestFeedSeedSites(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	anthropicAPIKey *string,
) (*SuggestFeedSeedSitesResponse, error) {
	return postWithHeaders[SuggestFeedSeedSitesResponse](ctx, w, "/suggest-feed-seed-sites", map[string]any{
		"existing_sources": existing,
		"preferred_topics": preferredTopics,
		"model":            nil,
	}, workerHeaders(anthropicAPIKey, nil, w.internalSecret))
}

func (w *WorkerClient) SuggestFeedSeedSitesWithModel(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	anthropicAPIKey *string,
	model *string,
) (*SuggestFeedSeedSitesResponse, error) {
	return postWithHeaders[SuggestFeedSeedSitesResponse](ctx, w, "/suggest-feed-seed-sites", map[string]any{
		"existing_sources": existing,
		"preferred_topics": preferredTopics,
		"model":            model,
	}, workerHeaders(anthropicAPIKey, nil, w.internalSecret))
}

func workerHeaders(anthropicAPIKey *string, googleAPIKey *string, internalSecret string) map[string]string {
	headers := map[string]string{}
	if internalSecret != "" {
		headers["X-Internal-Worker-Secret"] = internalSecret
	}
	if anthropicAPIKey != nil && *anthropicAPIKey != "" {
		headers["X-Anthropic-Api-Key"] = *anthropicAPIKey
	}
	if googleAPIKey != nil && *googleAPIKey != "" {
		headers["X-Google-Api-Key"] = *googleAPIKey
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func postWithHeaders[T any](ctx context.Context, w *WorkerClient, path string, body any, headers map[string]string) (*T, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := w.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(b) > 0 {
			return nil, fmt.Errorf("worker %s: status %d body=%s", path, resp.StatusCode, string(b))
		}
		return nil, fmt.Errorf("worker %s: status %d", path, resp.StatusCode)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
