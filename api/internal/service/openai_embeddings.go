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
	"strings"
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

func OpenRouterDescriptionTranslationModel() string {
	if v := os.Getenv("OPENROUTER_DESCRIPTION_TRANSLATION_MODEL"); v != "" {
		return v
	}
	return "gpt-5-nano"
}

func (c *OpenAIClient) TranslateTextsToJA(ctx context.Context, apiKey, model string, inputs map[string]string) (map[string]string, error) {
	if c == nil {
		return nil, fmt.Errorf("openai client is nil")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}
	if model == "" {
		model = OpenRouterDescriptionTranslationModel()
	}
	type requestItem struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	items := make([]requestItem, 0, len(inputs))
	for id, text := range inputs {
		if id == "" || text == "" {
			continue
		}
		items = append(items, requestItem{ID: id, Text: text})
	}
	if len(items) == 0 {
		return map[string]string{}, nil
	}
	promptJSON, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return nil, err
	}
	if shouldUseResponsesAPI(model) {
		return c.translateTextsToJAResponses(ctx, apiKey, model, promptJSON)
	}
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Translate each English model description into natural Japanese. Return only valid JSON. Preserve model names and technical tokens.",
			},
			{
				"role":    "user",
				"content": string(promptJSON),
			},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "translated_descriptions",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":         map[string]any{"type": "string"},
									"translated": map[string]any{"type": "string"},
								},
								"required":             []string{"id", "translated"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"items"},
					"additionalProperties": false,
				},
			},
		},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(b))
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
			return nil, fmt.Errorf("openai translate descriptions: status %d body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("openai translate descriptions: status %d", resp.StatusCode)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("openai translate descriptions: empty content")
	}
	var translated struct {
		Items []struct {
			ID         string `json:"id"`
			Translated string `json:"translated"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(decoded.Choices[0].Message.Content), &translated); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(translated.Items))
	for _, item := range translated.Items {
		if item.ID == "" || item.Translated == "" {
			continue
		}
		out[item.ID] = item.Translated
	}
	return out, nil
}

func (c *OpenAIClient) translateTextsToJAResponses(ctx context.Context, apiKey, model string, promptJSON []byte) (map[string]string, error) {
	reqBody := map[string]any{
		"model":             model,
		"input":             string(promptJSON),
		"instructions":      "Translate each English model description into natural Japanese. Return only valid JSON. Preserve model names and technical tokens.",
		"max_output_tokens": 1200,
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "translated_descriptions",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":         map[string]any{"type": "string"},
									"translated": map[string]any{"type": "string"},
								},
								"required":             []string{"id", "translated"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"items"},
					"additionalProperties": false,
				},
			},
		},
	}
	if reasoning := responsesReasoning(model); reasoning != nil {
		reqBody["reasoning"] = reasoning
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/responses", bytes.NewReader(b))
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
			return nil, fmt.Errorf("openai translate descriptions responses: status %d body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("openai translate descriptions responses: status %d", resp.StatusCode)
	}
	var decoded struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text       string `json:"text"`
				OutputText string `json:"output_text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(decoded.OutputText)
	if content == "" {
		var parts []string
		for _, item := range decoded.Output {
			for _, c := range item.Content {
				text := strings.TrimSpace(c.Text)
				if text == "" {
					text = strings.TrimSpace(c.OutputText)
				}
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
		content = strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if content == "" {
		return nil, fmt.Errorf("openai translate descriptions responses: empty content")
	}
	var translated struct {
		Items []struct {
			ID         string `json:"id"`
			Translated string `json:"translated"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(content), &translated); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(translated.Items))
	for _, item := range translated.Items {
		if item.ID == "" || item.Translated == "" {
			continue
		}
		out[item.ID] = item.Translated
	}
	return out, nil
}

func shouldUseResponsesAPI(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-5")
}

func responsesReasoning(model string) map[string]any {
	family := strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(family, "gpt-5") {
		return nil
	}
	if strings.HasSuffix(family, "-pro") {
		return nil
	}
	if strings.HasPrefix(family, "gpt-5.1") || strings.HasPrefix(family, "gpt-5.2") || strings.HasPrefix(family, "gpt-5.4") || strings.HasPrefix(family, "gpt-5.5") {
		return map[string]any{"effort": "none"}
	}
	return map[string]any{"effort": "minimal"}
}
