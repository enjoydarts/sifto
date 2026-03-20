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

type workerErrorDetailPayload struct {
	Detail any `json:"detail"`
}

type workerTraceMetaKey string

const (
	workerTraceUserIDKey   workerTraceMetaKey = "user_id"
	workerTracePurposeKey  workerTraceMetaKey = "purpose"
	workerTraceItemIDKey   workerTraceMetaKey = "item_id"
	workerTraceDigestIDKey workerTraceMetaKey = "digest_id"
	workerTraceSourceIDKey workerTraceMetaKey = "source_id"
)

func WithWorkerTraceMetadata(ctx context.Context, purpose string, userID, sourceID, itemID, digestID *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if userID != nil && *userID != "" {
		ctx = context.WithValue(ctx, workerTraceUserIDKey, *userID)
	}
	if purpose != "" {
		ctx = context.WithValue(ctx, workerTracePurposeKey, purpose)
	}
	if sourceID != nil && *sourceID != "" {
		ctx = context.WithValue(ctx, workerTraceSourceIDKey, *sourceID)
	}
	if itemID != nil && *itemID != "" {
		ctx = context.WithValue(ctx, workerTraceItemIDKey, *itemID)
	}
	if digestID != nil && *digestID != "" {
		ctx = context.WithValue(ctx, workerTraceDigestIDKey, *digestID)
	}
	return ctx
}

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
	composeTimeout := workerComposeDigestTimeout()
	httpTimeout := 60 * time.Second
	// Keep client timeout longer than compose timeout; otherwise http.Client.Timeout
	// can fire before context.WithTimeout in compose calls.
	if composeTimeout > 0 && composeTimeout+15*time.Second > httpTimeout {
		httpTimeout = composeTimeout + 15*time.Second
	}
	return &WorkerClient{
		baseURL:              url,
		http:                 &http.Client{Timeout: httpTimeout},
		composeDigestTimeout: composeTimeout,
		internalSecret:       strings.TrimSpace(os.Getenv("INTERNAL_WORKER_SECRET")),
	}
}

func workerComposeDigestTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("PYTHON_WORKER_COMPOSE_DIGEST_TIMEOUT_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 420 * time.Second
}

type ExtractBodyResponse struct {
	Title       *string `json:"title"`
	Content     string  `json:"content"`
	PublishedAt *string `json:"published_at"`
	ImageURL    *string `json:"image_url"`
}

type ExtractFactsResponse struct {
	Facts                []string  `json:"facts"`
	LLM                  *LLMUsage `json:"llm,omitempty"`
	FactsLocalizationLLM *LLMUsage `json:"facts_localization_llm,omitempty"`
}

type SummarizeResponse struct {
	Summary            string         `json:"summary"`
	Topics             []string       `json:"topics"`
	TranslatedTitle    string         `json:"translated_title,omitempty"`
	Score              float64        `json:"score"`
	ScoreBreakdown     map[string]any `json:"score_breakdown,omitempty"`
	ScoreReason        string         `json:"score_reason,omitempty"`
	ScorePolicyVersion string         `json:"score_policy_version,omitempty"`
	LLM                *LLMUsage      `json:"llm,omitempty"`
}

type SummaryFaithfulnessResponse struct {
	Verdict      string    `json:"verdict"`
	ShortComment string    `json:"short_comment"`
	LLM          *LLMUsage `json:"llm,omitempty"`
}

type FactsCheckResponse struct {
	Verdict      string    `json:"verdict"`
	ShortComment string    `json:"short_comment"`
	LLM          *LLMUsage `json:"llm,omitempty"`
}

type TranslateTitleResponse struct {
	TranslatedTitle string    `json:"translated_title"`
	LLM             *LLMUsage `json:"llm,omitempty"`
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
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Title         *string  `json:"title,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
	MatchedTopics []string `json:"matched_topics,omitempty"`
}

type RankFeedSuggestionsExistingSource struct {
	URL   string  `json:"url"`
	Title *string `json:"title,omitempty"`
}

type RankFeedSuggestionsExample struct {
	URL    string  `json:"url"`
	Title  *string `json:"title,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

type RankFeedSuggestionsItem struct {
	ID         *string `json:"id,omitempty"`
	URL        string  `json:"url"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

type RankFeedSuggestionsResponse struct {
	Items []RankFeedSuggestionsItem `json:"items"`
	LLM   *LLMUsage                 `json:"llm,omitempty"`
}

type AskCandidate struct {
	ItemID          string   `json:"item_id"`
	Title           *string  `json:"title,omitempty"`
	TranslatedTitle *string  `json:"translated_title,omitempty"`
	URL             string   `json:"url"`
	Summary         string   `json:"summary"`
	Facts           []string `json:"facts,omitempty"`
	Topics          []string `json:"topics,omitempty"`
	PublishedAt     *string  `json:"published_at,omitempty"`
	Similarity      float64  `json:"similarity"`
}

type AskCitation struct {
	ItemID string `json:"item_id"`
	Reason string `json:"reason"`
}

type AskResponse struct {
	Answer    string        `json:"answer"`
	Bullets   []string      `json:"bullets"`
	Citations []AskCitation `json:"citations"`
	LLM       *LLMUsage     `json:"llm,omitempty"`
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
	Provider                 string                `json:"provider"`
	Model                    string                `json:"model"`
	RequestedModel           string                `json:"requested_model,omitempty"`
	ResolvedModel            string                `json:"resolved_model,omitempty"`
	PricingModelFamily       string                `json:"pricing_model_family,omitempty"`
	PricingSource            string                `json:"pricing_source,omitempty"`
	OpenRouterCostUSD        *float64              `json:"openrouter_cost_usd,omitempty"`
	OpenRouterGenerationID   string                `json:"openrouter_generation_id,omitempty"`
	InputTokens              int                   `json:"input_tokens"`
	OutputTokens             int                   `json:"output_tokens"`
	CacheCreationInputTokens int                   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int                   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64               `json:"estimated_cost_usd"`
	ExecutionFailures        []LLMExecutionFailure `json:"execution_failures,omitempty"`
}

type LLMExecutionFailure struct {
	Model  string `json:"model"`
	Reason string `json:"reason"`
}

func (w *WorkerClient) ExtractBody(ctx context.Context, url string) (*ExtractBodyResponse, error) {
	return postWithHeaders[ExtractBodyResponse](ctx, w, "/extract-body", map[string]any{"url": url}, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
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

func (w *WorkerClient) ExtractFacts(ctx context.Context, title *string, content string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string) (*ExtractFactsResponse, error) {
	return postWithHeaders[ExtractFactsResponse](ctx, w, "/extract-facts", map[string]any{
		"title":   title,
		"content": content,
		"model":   nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) ExtractFactsWithModel(ctx context.Context, title *string, content string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*ExtractFactsResponse, error) {
	return postWithHeaders[ExtractFactsResponse](ctx, w, "/extract-facts", map[string]any{
		"title":   title,
		"content": content,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) Summarize(ctx context.Context, title *string, facts []string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title":             title,
		"facts":             facts,
		"model":             nil,
		"source_text_chars": nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) SummarizeWithModel(ctx context.Context, title *string, facts []string, sourceTextChars *int, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title":             title,
		"facts":             facts,
		"model":             model,
		"source_text_chars": sourceTextChars,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) CheckSummaryFaithfulnessWithModel(ctx context.Context, title *string, facts []string, summary string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*SummaryFaithfulnessResponse, error) {
	return postWithHeaders[SummaryFaithfulnessResponse](ctx, w, "/check-summary-faithfulness", map[string]any{
		"title":   title,
		"facts":   facts,
		"summary": summary,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) CheckFactsWithModel(ctx context.Context, title *string, content string, facts []string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*FactsCheckResponse, error) {
	return postWithHeaders[FactsCheckResponse](ctx, w, "/check-facts", map[string]any{
		"title":   title,
		"content": content,
		"facts":   facts,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) TranslateTitleWithModel(ctx context.Context, title string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*TranslateTitleResponse, error) {
	return postWithHeaders[TranslateTitleResponse](ctx, w, "/translate-title", map[string]any{
		"title": title,
		"model": model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) ComposeDigest(ctx context.Context, digestDate string, items []ComposeDigestItem, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string) (*ComposeDigestResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.composeDigestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.composeDigestTimeout)
		defer cancel()
	}
	return postWithHeaders[ComposeDigestResponse](ctx, w, "/compose-digest", map[string]any{
		"digest_date": digestDate,
		"items":       items,
		"model":       nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) ComposeDigestWithModel(ctx context.Context, digestDate string, items []ComposeDigestItem, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*ComposeDigestResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.composeDigestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.composeDigestTimeout)
		defer cancel()
	}
	return postWithHeaders[ComposeDigestResponse](ctx, w, "/compose-digest", map[string]any{
		"digest_date": digestDate,
		"items":       items,
		"model":       model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) ComposeDigestClusterDraftWithModel(
	ctx context.Context,
	clusterLabel string,
	itemCount int,
	topics []string,
	sourceLines []string,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	fireworksAPIKey *string,
	openAIAPIKey *string,
	model *string,
) (*ComposeDigestClusterDraftResponse, error) {
	return postWithHeaders[ComposeDigestClusterDraftResponse](ctx, w, "/compose-digest-cluster-draft", map[string]any{
		"cluster_label": clusterLabel,
		"item_count":    itemCount,
		"topics":        topics,
		"source_lines":  sourceLines,
		"model":         model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) AskWithModel(
	ctx context.Context,
	query string,
	candidates []AskCandidate,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	fireworksAPIKey *string,
	openAIAPIKey *string,
	model *string,
) (*AskResponse, error) {
	return postWithHeaders[AskResponse](ctx, w, "/ask", map[string]any{
		"query":      query,
		"candidates": candidates,
		"model":      model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) RankFeedSuggestions(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	candidates []RankFeedSuggestionsCandidate,
	positiveExamples []RankFeedSuggestionsExample,
	negativeExamples []RankFeedSuggestionsExample,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	fireworksAPIKey *string,
	openAIAPIKey *string,
) (*RankFeedSuggestionsResponse, error) {
	return postWithHeaders[RankFeedSuggestionsResponse](ctx, w, "/rank-feed-suggestions", map[string]any{
		"existing_sources":  existing,
		"preferred_topics":  preferredTopics,
		"candidates":        candidates,
		"positive_examples": positiveExamples,
		"negative_examples": negativeExamples,
		"model":             nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) RankFeedSuggestionsWithModel(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	candidates []RankFeedSuggestionsCandidate,
	positiveExamples []RankFeedSuggestionsExample,
	negativeExamples []RankFeedSuggestionsExample,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	fireworksAPIKey *string,
	openAIAPIKey *string,
	model *string,
) (*RankFeedSuggestionsResponse, error) {
	return postWithHeaders[RankFeedSuggestionsResponse](ctx, w, "/rank-feed-suggestions", map[string]any{
		"existing_sources":  existing,
		"preferred_topics":  preferredTopics,
		"candidates":        candidates,
		"positive_examples": positiveExamples,
		"negative_examples": negativeExamples,
		"model":             model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) SuggestFeedSeedSites(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	positiveExamples []RankFeedSuggestionsExample,
	negativeExamples []RankFeedSuggestionsExample,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	fireworksAPIKey *string,
	openAIAPIKey *string,
) (*SuggestFeedSeedSitesResponse, error) {
	return postWithHeaders[SuggestFeedSeedSitesResponse](ctx, w, "/suggest-feed-seed-sites", map[string]any{
		"existing_sources":  existing,
		"preferred_topics":  preferredTopics,
		"positive_examples": positiveExamples,
		"negative_examples": negativeExamples,
		"model":             nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func (w *WorkerClient) SuggestFeedSeedSitesWithModel(
	ctx context.Context,
	existing []RankFeedSuggestionsExistingSource,
	preferredTopics []string,
	positiveExamples []RankFeedSuggestionsExample,
	negativeExamples []RankFeedSuggestionsExample,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	fireworksAPIKey *string,
	openAIAPIKey *string,
	model *string,
) (*SuggestFeedSeedSitesResponse, error) {
	return postWithHeaders[SuggestFeedSeedSitesResponse](ctx, w, "/suggest-feed-seed-sites", map[string]any{
		"existing_sources":  existing,
		"preferred_topics":  preferredTopics,
		"positive_examples": positiveExamples,
		"negative_examples": negativeExamples,
		"model":             model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, w.internalSecret))
}

func workerHeaders(anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, internalSecret string) map[string]string {
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
	if groqAPIKey != nil && *groqAPIKey != "" {
		headers["X-Groq-Api-Key"] = *groqAPIKey
	}
	if deepseekAPIKey != nil && *deepseekAPIKey != "" {
		headers["X-Deepseek-Api-Key"] = *deepseekAPIKey
	}
	if alibabaAPIKey != nil && *alibabaAPIKey != "" {
		headers["X-Alibaba-Api-Key"] = *alibabaAPIKey
	}
	if mistralAPIKey != nil && *mistralAPIKey != "" {
		headers["X-Mistral-Api-Key"] = *mistralAPIKey
	}
	if xaiAPIKey != nil && *xaiAPIKey != "" {
		headers["X-Xai-Api-Key"] = *xaiAPIKey
	}
	if zaiAPIKey != nil && *zaiAPIKey != "" {
		headers["X-Zai-Api-Key"] = *zaiAPIKey
	}
	if fireworksAPIKey != nil && *fireworksAPIKey != "" {
		headers["X-Fireworks-Api-Key"] = *fireworksAPIKey
	}
	if openAIAPIKey != nil && *openAIAPIKey != "" {
		headers["X-Openai-Api-Key"] = *openAIAPIKey
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func applyWorkerTraceHeaders(ctx context.Context, headers map[string]string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	if ctx == nil {
		return headers
	}
	if v, _ := ctx.Value(workerTracePurposeKey).(string); v != "" {
		headers["X-Sifto-Purpose"] = v
	}
	if v, _ := ctx.Value(workerTraceUserIDKey).(string); v != "" {
		headers["X-Sifto-User-Id"] = v
	}
	if v, _ := ctx.Value(workerTraceSourceIDKey).(string); v != "" {
		headers["X-Sifto-Source-Id"] = v
	}
	if v, _ := ctx.Value(workerTraceItemIDKey).(string); v != "" {
		headers["X-Sifto-Item-Id"] = v
	}
	if v, _ := ctx.Value(workerTraceDigestIDKey).(string); v != "" {
		headers["X-Sifto-Digest-Id"] = v
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
	for k, v := range applyWorkerTraceHeaders(ctx, headers) {
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
			if detail := extractWorkerErrorDetail(b); detail != "" {
				return nil, fmt.Errorf("worker %s: status %d detail=%s", path, resp.StatusCode, detail)
			}
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

func extractWorkerErrorDetail(body []byte) string {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return ""
	}

	var payload workerErrorDetailPayload
	if err := json.Unmarshal(body, &payload); err == nil && payload.Detail != nil {
		switch v := payload.Detail.(type) {
		case string:
			return strings.TrimSpace(v)
		default:
			if b, err := json.Marshal(v); err == nil {
				return strings.TrimSpace(string(b))
			}
		}
	}

	if strings.EqualFold(raw, "Internal Server Error") {
		return ""
	}
	return raw
}
