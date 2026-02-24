package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"
)

type OpenAIClient struct {
	baseURL string
	http    *http.Client
}

func NewOpenAIClient() *OpenAIClient {
	baseURL := os.Getenv("OPENAI_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return &OpenAIClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func OpenAIEmbeddingModel() string {
	if v := os.Getenv("OPENAI_EMBEDDING_MODEL"); v != "" {
		return v
	}
	return "text-embedding-3-small"
}

type CreateEmbeddingResponse struct {
	Embedding []float64
	LLM       *LLMUsage
}

func (c *OpenAIClient) CreateEmbedding(ctx context.Context, apiKey, model, input string) (*CreateEmbeddingResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("openai client is nil")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}
	if model == "" {
		model = OpenAIEmbeddingModel()
	}
	reqBody := map[string]any{
		"model": model,
		"input": input,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(body) > 0 {
			return nil, fmt.Errorf("openai embeddings: status %d body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("openai embeddings: status %d", resp.StatusCode)
	}

	var decoded struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Data) == 0 || len(decoded.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai embeddings: empty embedding")
	}

	embedding := normalizeVector(decoded.Data[0].Embedding)
	cost, err := EstimateOpenAIEmbeddingCostUSD(model, decoded.Usage.PromptTokens)
	if err != nil {
		return nil, err
	}
	return &CreateEmbeddingResponse{
		Embedding: embedding,
		LLM: &LLMUsage{
			Provider:                 cost.Provider,
			Model:                    cost.Model,
			PricingModelFamily:       cost.PricingModelFamily,
			PricingSource:            cost.PricingSource,
			InputTokens:              cost.InputTokens,
			OutputTokens:             0,
			CacheCreationInputTokens: 0,
			CacheReadInputTokens:     0,
			EstimatedCostUSD:         cost.EstimatedCostUSD,
		},
	}, nil
}

func normalizeVector(v []float64) []float64 {
	if len(v) == 0 {
		return v
	}
	var normSq float64
	for _, x := range v {
		normSq += x * x
	}
	if normSq == 0 {
		return v
	}
	norm := math.Sqrt(normSq)
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}
