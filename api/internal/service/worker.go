package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type WorkerClient struct {
	baseURL string
	http    *http.Client
}

func NewWorkerClient() *WorkerClient {
	url := os.Getenv("PYTHON_WORKER_URL")
	if url == "" {
		url = "http://localhost:8000"
	}
	return &WorkerClient{
		baseURL: url,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

type ExtractBodyResponse struct {
	Title       *string `json:"title"`
	Content     string  `json:"content"`
	PublishedAt *string `json:"published_at"`
}

type ExtractFactsResponse struct {
	Facts []string `json:"facts"`
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
	Subject string `json:"subject"`
	Body    string `json:"body"`
	LLM     *LLMUsage `json:"llm,omitempty"`
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
	return postWithHeaders[ExtractBodyResponse](ctx, w, "/extract-body", map[string]any{"url": url}, nil)
}

func (w *WorkerClient) ExtractFacts(ctx context.Context, title *string, content string, anthropicAPIKey *string) (*ExtractFactsResponse, error) {
	return postWithHeaders[ExtractFactsResponse](ctx, w, "/extract-facts", map[string]any{
		"title":   title,
		"content": content,
	}, workerHeaders(anthropicAPIKey))
}

func (w *WorkerClient) Summarize(ctx context.Context, title *string, facts []string, anthropicAPIKey *string) (*SummarizeResponse, error) {
	return postWithHeaders[SummarizeResponse](ctx, w, "/summarize", map[string]any{
		"title": title,
		"facts": facts,
	}, workerHeaders(anthropicAPIKey))
}

func (w *WorkerClient) ComposeDigest(ctx context.Context, digestDate string, items []ComposeDigestItem, anthropicAPIKey *string) (*ComposeDigestResponse, error) {
	return postWithHeaders[ComposeDigestResponse](ctx, w, "/compose-digest", map[string]any{
		"digest_date": digestDate,
		"items":       items,
	}, workerHeaders(anthropicAPIKey))
}

func workerHeaders(anthropicAPIKey *string) map[string]string {
	if anthropicAPIKey == nil || *anthropicAPIKey == "" {
		return nil
	}
	return map[string]string{
		"X-Anthropic-Api-Key": *anthropicAPIKey,
	}
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
