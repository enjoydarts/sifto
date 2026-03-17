package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

type ProviderModelDiscoveryService struct {
	http *http.Client
}

type ProviderModelsResult struct {
	Provider string
	Models   []string
}

func NewProviderModelDiscoveryService() *ProviderModelDiscoveryService {
	return &ProviderModelDiscoveryService{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *ProviderModelDiscoveryService) DiscoverAll(ctx context.Context) ([]ProviderModelsResult, error) {
	providers := []struct {
		name string
		fn   func(context.Context) ([]string, error)
	}{
		{"openai", s.fetchOpenAIModels},
		{"anthropic", s.fetchAnthropicModels},
		{"google", s.fetchGoogleModels},
		{"groq", s.fetchGroqModels},
		{"deepseek", s.fetchDeepSeekModels},
		{"mistral", s.fetchMistralModels},
		{"xai", s.fetchXAIModels},
		{"fireworks", s.fetchFireworksModels},
	}
	out := make([]ProviderModelsResult, 0, len(providers))
	for _, p := range providers {
		models, err := p.fn(ctx)
		if err != nil {
			if strings.Contains(err.Error(), "api key is required") {
				continue
			}
			return nil, fmt.Errorf("%s model discovery: %w", p.name, err)
		}
		out = append(out, ProviderModelsResult{Provider: p.name, Models: models})
	}
	return out, nil
}

func normalizeModelIDs(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	slices.Sort(out)
	return out
}

func readJSONResponse(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if len(body) > 0 {
			return fmt.Errorf("status %d body=%s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (s *ProviderModelDiscoveryService) fetchOpenAIModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchAnthropicModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchGoogleModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	u, _ := url.Parse("https://generativelanguage.googleapis.com/v1beta/models")
	q := u.Query()
	q.Set("key", apiKey)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Models))
	for _, item := range decoded.Models {
		name := strings.TrimPrefix(item.Name, "models/")
		models = append(models, name)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchGroqModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchDeepSeekModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.deepseek.com/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchMistralModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("MISTRAL_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("MISTRAL_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.mistral.ai/v1"
	} else if strings.HasSuffix(base, "/chat/completions") {
		base = strings.TrimSuffix(base, "/chat/completions")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchXAIModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("XAI_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("XAI_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.x.ai/v1"
	} else if strings.HasSuffix(base, "/chat/completions") {
		base = strings.TrimSuffix(base, "/chat/completions")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

type fireworksModelListItem struct {
	Name                  string `json:"name"`
	DisplayName           string `json:"displayName"`
	Description           string `json:"description"`
	SupportsServerless    bool   `json:"supportsServerless"`
	SupportsServerlessAlt bool   `json:"supports_serverless"`
	BaseModelDetails      struct {
		ModelType string `json:"modelType"`
	} `json:"baseModelDetails"`
	ModelType string `json:"modelType"`
}

func isFireworksTextModel(item fireworksModelListItem) bool {
	combined := strings.ToLower(strings.Join([]string{
		item.Name,
		item.DisplayName,
		item.Description,
		item.BaseModelDetails.ModelType,
		item.ModelType,
	}, " "))
	if strings.TrimSpace(combined) == "" {
		return false
	}
	excludeKeywords := []string{
		"whisper",
		"speech",
		"audio",
		"transcription",
		"embedding",
		"embed",
		"rerank",
		"guard",
		"safeguard",
		"moderation",
		"vision",
		"image",
		"diffusion",
	}
	for _, keyword := range excludeKeywords {
		if strings.Contains(combined, keyword) {
			return false
		}
	}
	if strings.Contains(combined, "llm") || strings.Contains(combined, "text") || strings.Contains(combined, "chat") || strings.Contains(combined, "instruct") {
		return true
	}
	return true
}

func (s *ProviderModelDiscoveryService) fetchFireworksModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("FIREWORKS_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("FIREWORKS_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.fireworks.ai/inference/v1"
	}
	modelsURL := strings.TrimSuffix(base, "/chat/completions") + "/models?filter=supports_serverless=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var decoded struct {
		Data   []fireworksModelListItem `json:"data"`
		Models []fireworksModelListItem `json:"models"`
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := readJSONResponse(resp, &decoded); err != nil {
		return nil, err
	}
	items := decoded.Data
	if len(items) == 0 {
		items = decoded.Models
	}
	models := make([]string, 0, len(items))
	for _, item := range items {
		if !(item.SupportsServerless || item.SupportsServerlessAlt) {
			continue
		}
		if !isFireworksTextModel(item) {
			continue
		}
		models = append(models, strings.TrimSpace(item.Name))
	}
	return normalizeModelIDs(models), nil
}
