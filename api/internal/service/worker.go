package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	askTimeout           time.Duration
	audioBriefingTimeout time.Duration
	internalSecret       string
}

type AudioBriefingDeleteObjectsResponse struct {
	DeletedCount int `json:"deleted_count"`
}

type AudioBriefingCopyObjectsResponse struct {
	CopiedCount int `json:"copied_count"`
}

type AudioBriefingStatObjectResponse struct {
	SizeBytes int64 `json:"size_bytes"`
}

type AudioBriefingUploadObjectResponse struct {
	ObjectKey string `json:"object_key"`
}

func NewWorkerClient() *WorkerClient {
	url := os.Getenv("PYTHON_WORKER_URL")
	if url == "" {
		url = "http://localhost:8000"
	}
	composeTimeout := workerComposeDigestTimeout()
	askTimeout := workerAskTimeout()
	audioBriefingTimeout := workerAudioBriefingTimeout()
	httpTimeout := 60 * time.Second
	// Keep client timeout longer than compose timeout; otherwise http.Client.Timeout
	// can fire before context.WithTimeout in compose calls.
	if composeTimeout > 0 && composeTimeout+15*time.Second > httpTimeout {
		httpTimeout = composeTimeout + 15*time.Second
	}
	if askTimeout > 0 && askTimeout+15*time.Second > httpTimeout {
		httpTimeout = askTimeout + 15*time.Second
	}
	if audioBriefingTimeout > 0 && audioBriefingTimeout+15*time.Second > httpTimeout {
		httpTimeout = audioBriefingTimeout + 15*time.Second
	}
	return &WorkerClient{
		baseURL:              url,
		http:                 &http.Client{Timeout: httpTimeout},
		composeDigestTimeout: composeTimeout,
		askTimeout:           askTimeout,
		audioBriefingTimeout: audioBriefingTimeout,
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

func workerAskTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("PYTHON_WORKER_ASK_TIMEOUT_SEC")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 120 * time.Second
}

func workerAudioBriefingTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("PYTHON_WORKER_AUDIO_BRIEFING_TIMEOUT_SEC")); v != "" {
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

type ExtractBodyError struct {
	Message string
	Partial *ExtractBodyResponse
}

func (e *ExtractBodyError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func ExtractBodyPartial(err error) *ExtractBodyResponse {
	var extractErr *ExtractBodyError
	if !errors.As(err, &extractErr) || extractErr == nil {
		return nil
	}
	return extractErr.Partial
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

type BriefingNavigatorCandidate struct {
	ItemID          string   `json:"item_id"`
	Title           *string  `json:"title,omitempty"`
	TranslatedTitle *string  `json:"translated_title,omitempty"`
	SourceTitle     *string  `json:"source_title,omitempty"`
	Summary         string   `json:"summary"`
	Topics          []string `json:"topics,omitempty"`
	PublishedAt     *string  `json:"published_at,omitempty"`
	Score           *float64 `json:"score,omitempty"`
}

type BriefingNavigatorIntroContext struct {
	NowJST     string `json:"now_jst"`
	DateJST    string `json:"date_jst"`
	WeekdayJST string `json:"weekday_jst"`
	TimeOfDay  string `json:"time_of_day"`
	SeasonHint string `json:"season_hint"`
}

type BriefingNavigatorPick struct {
	ItemID     string   `json:"item_id"`
	Comment    string   `json:"comment"`
	ReasonTags []string `json:"reason_tags,omitempty"`
}

type BriefingNavigatorResponse struct {
	Intro string                  `json:"intro"`
	Picks []BriefingNavigatorPick `json:"picks"`
	LLM   *LLMUsage               `json:"llm,omitempty"`
}

type AINavigatorBriefItem struct {
	ItemID     string   `json:"item_id"`
	Comment    string   `json:"comment"`
	ReasonTags []string `json:"reason_tags,omitempty"`
}

type AINavigatorBriefResponse struct {
	Title   string                 `json:"title"`
	Intro   string                 `json:"intro"`
	Summary string                 `json:"summary"`
	Ending  string                 `json:"ending"`
	Items   []AINavigatorBriefItem `json:"items"`
	LLM     *LLMUsage              `json:"llm,omitempty"`
}

type SourceNavigatorCandidate struct {
	SourceID               string  `json:"source_id"`
	Title                  string  `json:"title"`
	URL                    string  `json:"url"`
	Enabled                bool    `json:"enabled"`
	Status                 string  `json:"status"`
	LastFetchedAt          *string `json:"last_fetched_at,omitempty"`
	LastItemAt             *string `json:"last_item_at,omitempty"`
	TotalItems30d          int     `json:"total_items_30d"`
	UnreadItems30d         int     `json:"unread_items_30d"`
	ReadItems30d           int     `json:"read_items_30d"`
	FavoriteCount30d       int     `json:"favorite_count_30d"`
	AvgItemsPerDay30d      float64 `json:"avg_items_per_day_30d"`
	ActiveDays30d          int     `json:"active_days_30d"`
	AvgItemsPerActiveDay30 float64 `json:"avg_items_per_active_day_30d"`
	FailureRate            float64 `json:"failure_rate"`
}

type SourceNavigatorPick struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Comment  string `json:"comment"`
}

type SourceNavigatorResponse struct {
	Overview string                `json:"overview"`
	Keep     []SourceNavigatorPick `json:"keep"`
	Watch    []SourceNavigatorPick `json:"watch"`
	Standout []SourceNavigatorPick `json:"standout"`
	LLM      *LLMUsage             `json:"llm,omitempty"`
}

type ItemNavigatorArticle struct {
	ItemID          string   `json:"item_id"`
	Title           *string  `json:"title,omitempty"`
	TranslatedTitle *string  `json:"translated_title,omitempty"`
	SourceTitle     *string  `json:"source_title,omitempty"`
	Summary         string   `json:"summary"`
	Facts           []string `json:"facts,omitempty"`
	PublishedAt     *string  `json:"published_at,omitempty"`
}

type ItemNavigatorResponse struct {
	Headline   string    `json:"headline"`
	Commentary string    `json:"commentary"`
	StanceTags []string  `json:"stance_tags,omitempty"`
	LLM        *LLMUsage `json:"llm,omitempty"`
}

type AudioBriefingScriptArticle struct {
	ItemID          string  `json:"item_id"`
	Title           *string `json:"title,omitempty"`
	TranslatedTitle *string `json:"translated_title,omitempty"`
	SourceTitle     *string `json:"source_title,omitempty"`
	Summary         string  `json:"summary"`
	PublishedAt     *string `json:"published_at,omitempty"`
}

type AudioBriefingScriptSegment struct {
	ItemID       string `json:"item_id"`
	Headline     string `json:"headline"`
	SummaryIntro string `json:"summary_intro"`
	Commentary   string `json:"commentary"`
}

type AudioBriefingScriptTurn struct {
	Speaker string  `json:"speaker"`
	Section string  `json:"section"`
	ItemID  *string `json:"item_id,omitempty"`
	Text    string  `json:"text"`
}

type AudioBriefingScriptResponse struct {
	Opening         string                       `json:"opening"`
	OverallSummary  string                       `json:"overall_summary"`
	ArticleSegments []AudioBriefingScriptSegment `json:"article_segments"`
	Turns           []AudioBriefingScriptTurn    `json:"turns,omitempty"`
	Ending          string                       `json:"ending"`
	LLM             *LLMUsage                    `json:"llm,omitempty"`
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

type AskNavigatorCitation struct {
	ItemID      string   `json:"item_id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Reason      string   `json:"reason,omitempty"`
	PublishedAt *string  `json:"published_at,omitempty"`
	Topics      []string `json:"topics,omitempty"`
}

type AskNavigatorRelatedItem struct {
	ItemID          string   `json:"item_id"`
	Title           *string  `json:"title,omitempty"`
	TranslatedTitle *string  `json:"translated_title,omitempty"`
	URL             string   `json:"url"`
	Summary         string   `json:"summary"`
	Topics          []string `json:"topics,omitempty"`
	PublishedAt     *string  `json:"published_at,omitempty"`
}

type AskNavigatorInput struct {
	Query        string                    `json:"query"`
	Answer       string                    `json:"answer"`
	Bullets      []string                  `json:"bullets,omitempty"`
	Citations    []AskNavigatorCitation    `json:"citations,omitempty"`
	RelatedItems []AskNavigatorRelatedItem `json:"related_items,omitempty"`
}

type AskNavigatorResponse struct {
	Headline   string    `json:"headline"`
	Commentary string    `json:"commentary"`
	NextAngles []string  `json:"next_angles,omitempty"`
	LLM        *LLMUsage `json:"llm,omitempty"`
}

type AudioBriefingSynthesizeUploadResponse struct {
	AudioObjectKey string `json:"audio_object_key"`
	DurationSec    int    `json:"duration_sec"`
}

type AudioBriefingGeminiDuoTurn struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type SummaryAudioSynthesizeResponse struct {
	AudioBase64  string `json:"audio_base64"`
	ContentType  string `json:"content_type"`
	DurationSec  int    `json:"duration_sec"`
	ResolvedText string `json:"resolved_text"`
}

type TTSMarkupPreprocessResponse struct {
	Text string    `json:"text"`
	LLM  *LLMUsage `json:"llm,omitempty"`
}

type AudioBriefingPresignResponse struct {
	AudioURL string `json:"audio_url"`
}

type SuggestFeedSeedSitesItem struct {
	URL    string  `json:"url"`
	Title  *string `json:"title,omitempty"`
	Reason string  `json:"reason"`
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

type PromptConfig struct {
	PromptKey         string  `json:"prompt_key,omitempty"`
	PromptSource      string  `json:"prompt_source,omitempty"`
	PromptText        string  `json:"prompt_text,omitempty"`
	SystemInstruction string  `json:"system_instruction,omitempty"`
	PromptVersionID   *string `json:"prompt_version_id,omitempty"`
	PromptVersion     *int    `json:"prompt_version_number,omitempty"`
}

func (w *WorkerClient) ExtractBody(ctx context.Context, url string) (*ExtractBodyResponse, error) {
	b, err := json.Marshal(map[string]any{"url": url})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.baseURL+"/extract-body", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range applyWorkerTraceHeaders(ctx, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret)) {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(body) > 0 {
			if detail := extractWorkerErrorDetail(body); detail != "" {
				return nil, &ExtractBodyError{
					Message: fmt.Sprintf("worker /extract-body: status %d detail=%s", resp.StatusCode, detail),
					Partial: extractBodyPartialFromError(body),
				}
			}
			return nil, &ExtractBodyError{Message: fmt.Sprintf("worker /extract-body: status %d body=%s", resp.StatusCode, string(body))}
		}
		return nil, &ExtractBodyError{Message: fmt.Sprintf("worker /extract-body: status %d", resp.StatusCode)}
	}

	var result ExtractBodyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) ExtractFactsWithModel(ctx context.Context, title *string, content string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string, prompt *PromptConfig) (*ExtractFactsResponse, error) {
	return postWithHeaders[ExtractFactsResponse](ctx, w, "/extract-facts", map[string]any{
		"title":   title,
		"content": content,
		"model":   model,
		"prompt":  prompt,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) Summarize(ctx context.Context, title *string, facts []string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title":             title,
		"facts":             facts,
		"model":             nil,
		"source_text_chars": nil,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) SummarizeWithModel(ctx context.Context, title *string, facts []string, sourceTextChars *int, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string, prompt *PromptConfig) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title":             title,
		"facts":             facts,
		"model":             model,
		"source_text_chars": sourceTextChars,
		"prompt":            prompt,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) CheckSummaryFaithfulnessWithModel(ctx context.Context, title *string, facts []string, summary string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*SummaryFaithfulnessResponse, error) {
	return postWithHeaders[SummaryFaithfulnessResponse](ctx, w, "/check-summary-faithfulness", map[string]any{
		"title":   title,
		"facts":   facts,
		"summary": summary,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) CheckFactsWithModel(ctx context.Context, title *string, content string, facts []string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*FactsCheckResponse, error) {
	return postWithHeaders[FactsCheckResponse](ctx, w, "/check-facts", map[string]any{
		"title":   title,
		"content": content,
		"facts":   facts,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) TranslateTitleWithModel(ctx context.Context, title string, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string) (*TranslateTitleResponse, error) {
	return postWithHeaders[TranslateTitleResponse](ctx, w, "/translate-title", map[string]any{
		"title": title,
		"model": model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) ComposeDigestWithModel(ctx context.Context, digestDate string, items []ComposeDigestItem, anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, model *string, prompt *PromptConfig) (*ComposeDigestResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.composeDigestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.composeDigestTimeout)
		defer cancel()
	}
	return postWithHeaders[ComposeDigestResponse](ctx, w, "/compose-digest", map[string]any{
		"digest_date": digestDate,
		"items":       items,
		"model":       model,
		"prompt":      prompt,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
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
	if _, ok := ctx.Deadline(); !ok && w.askTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.askTimeout)
		defer cancel()
	}
	return postWithHeaders[AskResponse](ctx, w, "/ask", map[string]any{
		"query":      query,
		"candidates": candidates,
		"model":      model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) GenerateAskNavigatorWithModel(
	ctx context.Context,
	persona string,
	input AskNavigatorInput,
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
) (*AskNavigatorResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.askTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.askTimeout)
		defer cancel()
	}
	return postWithHeaders[AskNavigatorResponse](ctx, w, "/ask-navigator", map[string]any{
		"persona": persona,
		"input":   input,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
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
	togetherAPIKey *string,
	moonshotAPIKey *string,
	openRouterAPIKey *string,
	poeAPIKey *string,
	siliconFlowAPIKey *string,
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, selectOpenAICompatibleKey(model, togetherAPIKey, moonshotAPIKey, openRouterAPIKey, poeAPIKey, siliconFlowAPIKey, openAIAPIKey), nil, nil, nil, w.internalSecret))
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
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
	togetherAPIKey *string,
	moonshotAPIKey *string,
	openRouterAPIKey *string,
	poeAPIKey *string,
	siliconFlowAPIKey *string,
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
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, selectOpenAICompatibleKey(model, togetherAPIKey, moonshotAPIKey, openRouterAPIKey, poeAPIKey, siliconFlowAPIKey, openAIAPIKey), nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) GenerateBriefingNavigatorWithModel(
	ctx context.Context,
	persona string,
	candidates []BriefingNavigatorCandidate,
	introContext BriefingNavigatorIntroContext,
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
) (*BriefingNavigatorResponse, error) {
	return postWithHeaders[BriefingNavigatorResponse](ctx, w, "/briefing-navigator", map[string]any{
		"persona":       persona,
		"candidates":    candidates,
		"intro_context": introContext,
		"model":         model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) ComposeAINavigatorBriefWithModel(
	ctx context.Context,
	persona string,
	candidates []BriefingNavigatorCandidate,
	introContext BriefingNavigatorIntroContext,
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
) (*AINavigatorBriefResponse, error) {
	return postWithHeaders[AINavigatorBriefResponse](ctx, w, "/ai-navigator-brief", map[string]any{
		"persona":       persona,
		"candidates":    candidates,
		"intro_context": introContext,
		"model":         model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) GenerateSourceNavigatorWithModel(
	ctx context.Context,
	persona string,
	candidates []SourceNavigatorCandidate,
	anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey string,
	model *string,
) (*SourceNavigatorResponse, error) {
	headers := map[string]string{
		"X-LLM-Provider": LLMProviderForModel(model),
		"X-LLM-Model":    strings.TrimSpace(derefString(model)),
	}
	if w.internalSecret != "" {
		headers["X-Internal-Worker-Secret"] = w.internalSecret
	}
	if anthropicAPIKey != "" {
		headers["X-Anthropic-Api-Key"] = anthropicAPIKey
	}
	if googleAPIKey != "" {
		headers["X-Google-Api-Key"] = googleAPIKey
	}
	if groqAPIKey != "" {
		headers["X-Groq-Api-Key"] = groqAPIKey
	}
	if deepseekAPIKey != "" {
		headers["X-DeepSeek-Api-Key"] = deepseekAPIKey
	}
	if alibabaAPIKey != "" {
		headers["X-Alibaba-Api-Key"] = alibabaAPIKey
	}
	if mistralAPIKey != "" {
		headers["X-Mistral-Api-Key"] = mistralAPIKey
	}
	if xaiAPIKey != "" {
		headers["X-XAI-Api-Key"] = xaiAPIKey
	}
	if zaiAPIKey != "" {
		headers["X-ZAI-Api-Key"] = zaiAPIKey
	}
	if fireworksAPIKey != "" {
		headers["X-Fireworks-Api-Key"] = fireworksAPIKey
	}
	if openAIAPIKey != "" {
		headers["X-OpenAI-Api-Key"] = openAIAPIKey
	}
	return postWithHeaders[SourceNavigatorResponse](ctx, w, "/source-navigator", map[string]any{
		"persona":    persona,
		"candidates": candidates,
		"model":      model,
	}, headers)
}

func (w *WorkerClient) GenerateItemNavigatorWithModel(
	ctx context.Context,
	persona string,
	article ItemNavigatorArticle,
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
) (*ItemNavigatorResponse, error) {
	return postWithHeaders[ItemNavigatorResponse](ctx, w, "/item-navigator", map[string]any{
		"persona": persona,
		"article": article,
		"model":   model,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) GenerateAudioBriefingScriptWithModel(
	ctx context.Context,
	persona string,
	conversationMode string,
	hostPersona *string,
	partnerPersona *string,
	articles []AudioBriefingScriptArticle,
	introContext map[string]any,
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
	targetDurationMinutes int,
	targetChars int,
	charsPerMinute int,
	includeOpening bool,
	includeOverallSummary bool,
	includeArticleSegments bool,
	includeEnding bool,
	prompt *PromptConfig,
) (*AudioBriefingScriptResponse, error) {
	return postWithHeaders[AudioBriefingScriptResponse](ctx, w, "/audio-briefing-script", map[string]any{
		"persona":                  persona,
		"conversation_mode":        conversationMode,
		"host_persona":             hostPersona,
		"partner_persona":          partnerPersona,
		"articles":                 articles,
		"intro_context":            introContext,
		"model":                    model,
		"target_duration_minutes":  targetDurationMinutes,
		"target_chars":             targetChars,
		"chars_per_minute":         charsPerMinute,
		"include_opening":          includeOpening,
		"include_overall_summary":  includeOverallSummary,
		"include_article_segments": includeArticleSegments,
		"include_ending":           includeEnding,
		"prompt":                   prompt,
	}, workerHeaders(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, fireworksAPIKey, openAIAPIKey, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) SynthesizeAudioBriefingUpload(
	ctx context.Context,
	provider string,
	voiceModel string,
	voiceStyle string,
	ttsModel string,
	azureSpeechRegion string,
	persona string,
	text string,
	speechRate float64,
	emotionalIntensity float64,
	tempoDynamics float64,
	lineBreakSilenceSeconds float64,
	chunkTrailingSilenceSeconds float64,
	pitch float64,
	volumeGain float64,
	outputObjectKey string,
	chunkID string,
	heartbeatURL string,
	heartbeatToken string,
	aivisUserDictionaryUUID *string,
	aivisAPIKey *string,
	fishAudioAPIKey *string,
	elevenLabsAPIKey *string,
	googleAPIKey *string,
	xaiAPIKey *string,
	openAIAPIKey *string,
	azureSpeechAPIKey *string,
) (*AudioBriefingSynthesizeUploadResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.audioBriefingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.audioBriefingTimeout)
		defer cancel()
	}
	requestBody := map[string]any{
		"provider":                       provider,
		"voice_model":                    voiceModel,
		"voice_style":                    voiceStyle,
		"tts_model":                      ttsModel,
		"azure_speech_region":            strings.TrimSpace(azureSpeechRegion),
		"persona":                        persona,
		"text":                           text,
		"speech_rate":                    speechRate,
		"emotional_intensity":            emotionalIntensity,
		"tempo_dynamics":                 tempoDynamics,
		"line_break_silence_seconds":     lineBreakSilenceSeconds,
		"chunk_trailing_silence_seconds": chunkTrailingSilenceSeconds,
		"pitch":                          pitch,
		"volume_gain":                    volumeGain,
		"output_object_key":              outputObjectKey,
		"chunk_id":                       strings.TrimSpace(chunkID),
		"heartbeat_url":                  strings.TrimSpace(heartbeatURL),
		"heartbeat_token":                strings.TrimSpace(heartbeatToken),
	}
	if uuid := strings.TrimSpace(derefString(aivisUserDictionaryUUID)); uuid != "" {
		requestBody["user_dictionary_uuid"] = uuid
	}
	headers := workerHeaders(nil, googleAPIKey, nil, nil, nil, nil, xaiAPIKey, nil, nil, openAIAPIKey, aivisAPIKey, fishAudioAPIKey, elevenLabsAPIKey, w.internalSecret)
	if azureSpeechAPIKey != nil && strings.TrimSpace(*azureSpeechAPIKey) != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		headers["X-Azure-Speech-Api-Key"] = strings.TrimSpace(*azureSpeechAPIKey)
	}
	return postWithHeaders[AudioBriefingSynthesizeUploadResponse](ctx, w, "/audio-briefing/synthesize-upload", requestBody, headers)
}

func (w *WorkerClient) SynthesizeAudioBriefingGeminiDuoUpload(
	ctx context.Context,
	ttsModel string,
	hostPersona string,
	partnerPersona string,
	hostVoiceModel string,
	partnerVoiceModel string,
	sectionType string,
	turns []AudioBriefingGeminiDuoTurn,
	outputObjectKey string,
	googleAPIKey *string,
) (*AudioBriefingSynthesizeUploadResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.audioBriefingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.audioBriefingTimeout)
		defer cancel()
	}
	requestBody := map[string]any{
		"tts_model":           strings.TrimSpace(ttsModel),
		"host_persona":        strings.TrimSpace(hostPersona),
		"partner_persona":     strings.TrimSpace(partnerPersona),
		"host_voice_model":    strings.TrimSpace(hostVoiceModel),
		"partner_voice_model": strings.TrimSpace(partnerVoiceModel),
		"section_type":        strings.TrimSpace(sectionType),
		"turns":               turns,
		"output_object_key":   outputObjectKey,
	}
	return postWithHeaders[AudioBriefingSynthesizeUploadResponse](ctx, w, "/audio-briefing/synthesize-upload-gemini-duo", requestBody, workerHeaders(nil, googleAPIKey, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) SynthesizeAudioBriefingFishDuoUpload(
	ctx context.Context,
	ttsModel string,
	hostPersona string,
	partnerPersona string,
	hostVoiceModel string,
	partnerVoiceModel string,
	sectionType string,
	turns []AudioBriefingGeminiDuoTurn,
	preprocessedText string,
	outputObjectKey string,
	fishAPIKey *string,
) (*AudioBriefingSynthesizeUploadResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.audioBriefingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.audioBriefingTimeout)
		defer cancel()
	}
	requestBody := map[string]any{
		"tts_model":           strings.TrimSpace(ttsModel),
		"host_persona":        strings.TrimSpace(hostPersona),
		"partner_persona":     strings.TrimSpace(partnerPersona),
		"host_voice_model":    strings.TrimSpace(hostVoiceModel),
		"partner_voice_model": strings.TrimSpace(partnerVoiceModel),
		"section_type":        strings.TrimSpace(sectionType),
		"turns":               turns,
		"output_object_key":   outputObjectKey,
	}
	if strings.TrimSpace(preprocessedText) != "" {
		requestBody["preprocessed_text"] = strings.TrimSpace(preprocessedText)
	}
	return postWithHeaders[AudioBriefingSynthesizeUploadResponse](ctx, w, "/audio-briefing/synthesize-upload-fish-duo", requestBody, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fishAPIKey, nil, w.internalSecret))
}

func (w *WorkerClient) SynthesizeAudioBriefingElevenLabsDuoUpload(
	ctx context.Context,
	ttsModel string,
	hostPersona string,
	partnerPersona string,
	hostVoiceModel string,
	partnerVoiceModel string,
	sectionType string,
	turns []AudioBriefingGeminiDuoTurn,
	outputObjectKey string,
	elevenLabsAPIKey *string,
) (*AudioBriefingSynthesizeUploadResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.audioBriefingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.audioBriefingTimeout)
		defer cancel()
	}
	requestBody := map[string]any{
		"tts_model":           strings.TrimSpace(ttsModel),
		"host_persona":        strings.TrimSpace(hostPersona),
		"partner_persona":     strings.TrimSpace(partnerPersona),
		"host_voice_model":    strings.TrimSpace(hostVoiceModel),
		"partner_voice_model": strings.TrimSpace(partnerVoiceModel),
		"section_type":        strings.TrimSpace(sectionType),
		"turns":               turns,
		"output_object_key":   outputObjectKey,
	}
	return postWithHeaders[AudioBriefingSynthesizeUploadResponse](ctx, w, "/audio-briefing/synthesize-upload-elevenlabs-duo", requestBody, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, elevenLabsAPIKey, w.internalSecret))
}

func (w *WorkerClient) SynthesizeAudioBriefingAzureSpeechDuoUpload(
	ctx context.Context,
	hostVoiceModel string,
	partnerVoiceModel string,
	sectionType string,
	turns []AudioBriefingGeminiDuoTurn,
	preprocessedText string,
	outputObjectKey string,
	speechRate float64,
	lineBreakSilenceSeconds float64,
	pitch float64,
	volumeGain float64,
	azureSpeechRegion string,
	azureSpeechAPIKey *string,
) (*AudioBriefingSynthesizeUploadResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.audioBriefingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.audioBriefingTimeout)
		defer cancel()
	}
	requestBody := map[string]any{
		"host_voice_model":           strings.TrimSpace(hostVoiceModel),
		"partner_voice_model":        strings.TrimSpace(partnerVoiceModel),
		"section_type":               strings.TrimSpace(sectionType),
		"turns":                      turns,
		"output_object_key":          outputObjectKey,
		"speech_rate":                speechRate,
		"line_break_silence_seconds": lineBreakSilenceSeconds,
		"pitch":                      pitch,
		"volume_gain":                volumeGain,
		"azure_speech_region":        strings.TrimSpace(azureSpeechRegion),
	}
	if strings.TrimSpace(preprocessedText) != "" {
		requestBody["preprocessed_text"] = strings.TrimSpace(preprocessedText)
	}
	headers := workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret)
	if azureSpeechAPIKey != nil && strings.TrimSpace(*azureSpeechAPIKey) != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		headers["X-Azure-Speech-Api-Key"] = strings.TrimSpace(*azureSpeechAPIKey)
	}
	return postWithHeaders[AudioBriefingSynthesizeUploadResponse](ctx, w, "/audio-briefing/synthesize-upload-azure-speech-duo", requestBody, headers)
}

func (w *WorkerClient) SynthesizeSummaryAudio(
	ctx context.Context,
	provider string,
	voiceModel string,
	voiceStyle string,
	ttsModel string,
	azureSpeechRegion string,
	text string,
	speechRate float64,
	emotionalIntensity float64,
	tempoDynamics float64,
	lineBreakSilenceSeconds float64,
	chunkTrailingSilenceSeconds float64,
	pitch float64,
	volumeGain float64,
	aivisUserDictionaryUUID *string,
	aivisAPIKey *string,
	fishAudioAPIKey *string,
	elevenLabsAPIKey *string,
	googleAPIKey *string,
	xaiAPIKey *string,
	openAIAPIKey *string,
	azureSpeechAPIKey *string,
) (*SummaryAudioSynthesizeResponse, error) {
	if _, ok := ctx.Deadline(); !ok && w.audioBriefingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.audioBriefingTimeout)
		defer cancel()
	}
	requestBody := map[string]any{
		"provider":                       provider,
		"voice_model":                    voiceModel,
		"voice_style":                    voiceStyle,
		"tts_model":                      ttsModel,
		"azure_speech_region":            strings.TrimSpace(azureSpeechRegion),
		"text":                           text,
		"speech_rate":                    speechRate,
		"emotional_intensity":            emotionalIntensity,
		"tempo_dynamics":                 tempoDynamics,
		"line_break_silence_seconds":     lineBreakSilenceSeconds,
		"chunk_trailing_silence_seconds": chunkTrailingSilenceSeconds,
		"pitch":                          pitch,
		"volume_gain":                    volumeGain,
	}
	if uuid := strings.TrimSpace(derefString(aivisUserDictionaryUUID)); uuid != "" {
		requestBody["user_dictionary_uuid"] = uuid
	}
	headers := workerHeaders(nil, googleAPIKey, nil, nil, nil, nil, xaiAPIKey, nil, nil, openAIAPIKey, aivisAPIKey, fishAudioAPIKey, elevenLabsAPIKey, w.internalSecret)
	if azureSpeechAPIKey != nil && strings.TrimSpace(*azureSpeechAPIKey) != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		headers["X-Azure-Speech-Api-Key"] = strings.TrimSpace(*azureSpeechAPIKey)
	}
	return postWithHeaders[SummaryAudioSynthesizeResponse](ctx, w, "/summary-audio/synthesize", requestBody, headers)
}

func (w *WorkerClient) PreprocessTTSMarkupText(
	ctx context.Context,
	text string,
	model string,
	promptKey string,
	variables map[string]string,
	apiKey *string,
) (*TTSMarkupPreprocessResponse, error) {
	if variables == nil {
		variables = map[string]string{}
	}
	requestBody := map[string]any{
		"text":       text,
		"model":      model,
		"prompt_key": promptKey,
		"variables":  variables,
	}
	headers := workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret)
	if headers == nil {
		headers = map[string]string{}
	}
	if apiKey != nil && *apiKey != "" {
		if provider := CatalogProviderForModel(model); provider != "" {
			if providerConfig := providerCatalogByID(provider); providerConfig != nil && providerConfig.APIKeyHeader != "" {
				headers[providerConfig.APIKeyHeader] = *apiKey
			}
		}
	}
	return postWithHeaders[TTSMarkupPreprocessResponse](ctx, w, "/tts/preprocess-text", requestBody, headers)
}

func (w *WorkerClient) PresignAudioBriefingObject(ctx context.Context, objectKey string, expiresSec int) (*AudioBriefingPresignResponse, error) {
	return w.PresignAudioBriefingObjectInBucket(ctx, objectKey, "", expiresSec)
}

func (w *WorkerClient) PresignAudioBriefingObjectInBucket(ctx context.Context, objectKey string, bucket string, expiresSec int) (*AudioBriefingPresignResponse, error) {
	return postWithHeaders[AudioBriefingPresignResponse](ctx, w, "/audio-briefing/presign", map[string]any{
		"object_key":  objectKey,
		"bucket":      bucket,
		"expires_sec": expiresSec,
	}, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) DeleteAudioBriefingObjects(ctx context.Context, objectRefs []AudioBriefingObjectRef) error {
	grouped := groupAudioBriefingObjectRefsByBucket(objectRefs)
	for bucket, objectKeys := range grouped {
		if err := w.DeleteAudioBriefingObjectsInBucket(ctx, bucket, objectKeys); err != nil {
			return err
		}
	}
	return nil
}

func (w *WorkerClient) DeleteAudioBriefingObjectsInBucket(ctx context.Context, bucket string, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}
	_, err := postWithHeaders[AudioBriefingDeleteObjectsResponse](ctx, w, "/audio-briefing/delete-objects", map[string]any{
		"object_keys": objectKeys,
		"bucket":      bucket,
	}, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
	return err
}

func (w *WorkerClient) CopyAudioBriefingObjects(ctx context.Context, sourceBucket string, targetBucket string, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}
	_, err := postWithHeaders[AudioBriefingCopyObjectsResponse](ctx, w, "/audio-briefing/copy-objects", map[string]any{
		"source_bucket": sourceBucket,
		"target_bucket": targetBucket,
		"object_keys":   objectKeys,
	}, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
	return err
}

func (w *WorkerClient) StatAudioBriefingObject(ctx context.Context, bucket string, objectKey string) (*AudioBriefingStatObjectResponse, error) {
	return postWithHeaders[AudioBriefingStatObjectResponse](ctx, w, "/audio-briefing/stat-object", map[string]any{
		"bucket":     bucket,
		"object_key": objectKey,
	}, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
}

func (w *WorkerClient) UploadAudioBriefingObject(ctx context.Context, bucket string, objectKey string, contentBase64 string, contentType string) (*AudioBriefingUploadObjectResponse, error) {
	return postWithHeaders[AudioBriefingUploadObjectResponse](ctx, w, "/audio-briefing/upload-object", map[string]any{
		"bucket":         bucket,
		"object_key":     objectKey,
		"content_base64": contentBase64,
		"content_type":   contentType,
	}, workerHeaders(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, w.internalSecret))
}

func workerHeaders(anthropicAPIKey *string, googleAPIKey *string, groqAPIKey *string, deepseekAPIKey *string, alibabaAPIKey *string, mistralAPIKey *string, xaiAPIKey *string, zaiAPIKey *string, fireworksAPIKey *string, openAIAPIKey *string, aivisAPIKey *string, fishAudioAPIKey *string, elevenLabsAPIKey *string, internalSecret string) map[string]string {
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
	if aivisAPIKey != nil && *aivisAPIKey != "" {
		headers["X-Aivis-Api-Key"] = *aivisAPIKey
	}
	if fishAudioAPIKey != nil && *fishAudioAPIKey != "" {
		headers["X-Fish-Api-Key"] = *fishAudioAPIKey
	}
	if elevenLabsAPIKey != nil && *elevenLabsAPIKey != "" {
		headers["X-Elevenlabs-Api-Key"] = *elevenLabsAPIKey
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func selectOpenAICompatibleKey(model *string, togetherAPIKey *string, moonshotAPIKey *string, openRouterAPIKey *string, poeAPIKey *string, siliconFlowAPIKey *string, openAIAPIKey *string) *string {
	switch LLMProviderForModel(model) {
	case "together":
		if togetherAPIKey != nil && strings.TrimSpace(*togetherAPIKey) != "" {
			return togetherAPIKey
		}
	case "moonshot":
		if moonshotAPIKey != nil && strings.TrimSpace(*moonshotAPIKey) != "" {
			return moonshotAPIKey
		}
	case "openrouter":
		if openRouterAPIKey != nil && strings.TrimSpace(*openRouterAPIKey) != "" {
			return openRouterAPIKey
		}
	case "poe":
		if poeAPIKey != nil && strings.TrimSpace(*poeAPIKey) != "" {
			return poeAPIKey
		}
	case "siliconflow":
		if siliconFlowAPIKey != nil && strings.TrimSpace(*siliconFlowAPIKey) != "" {
			return siliconFlowAPIKey
		}
	}
	return openAIAPIKey
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
		case map[string]any:
			if msg := strings.TrimSpace(fmt.Sprint(v["message"])); msg != "" && msg != "<nil>" {
				return msg
			}
			if b, err := json.Marshal(v); err == nil {
				return strings.TrimSpace(string(b))
			}
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

func extractBodyPartialFromError(body []byte) *ExtractBodyResponse {
	var payload workerErrorDetailPayload
	if err := json.Unmarshal(body, &payload); err != nil || payload.Detail == nil {
		return nil
	}
	detailMap, ok := payload.Detail.(map[string]any)
	if !ok {
		return nil
	}
	code := strings.TrimSpace(fmt.Sprint(detailMap["code"]))
	if code != "youtube_transcript_unavailable" {
		return nil
	}
	var result ExtractBodyResponse
	if title := strings.TrimSpace(fmt.Sprint(detailMap["title"])); title != "" && title != "<nil>" {
		result.Title = &title
	}
	if publishedAt := strings.TrimSpace(fmt.Sprint(detailMap["published_at"])); publishedAt != "" && publishedAt != "<nil>" {
		result.PublishedAt = &publishedAt
	}
	if imageURL := strings.TrimSpace(fmt.Sprint(detailMap["image_url"])); imageURL != "" && imageURL != "<nil>" {
		result.ImageURL = &imageURL
	}
	if result.Title == nil && result.PublishedAt == nil && result.ImageURL == nil {
		return nil
	}
	return &result
}
