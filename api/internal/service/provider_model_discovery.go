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
	keys ProviderModelDiscoveryKeys
}

type ProviderModelsResult struct {
	Provider string
	Models   []string
}

type ProviderModelDiscoveryKeys struct {
	OpenAI      string
	Anthropic   string
	Google      string
	Groq        string
	DeepSeek    string
	Alibaba     string
	Mistral     string
	Moonshot    string
	SiliconFlow string
	XAI         string
	ZAI         string
	Poe         string
	Fireworks   string
	Together    string
}

func NewProviderModelDiscoveryService() *ProviderModelDiscoveryService {
	return &ProviderModelDiscoveryService{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func NewProviderModelDiscoveryServiceWithKeys(keys ProviderModelDiscoveryKeys) *ProviderModelDiscoveryService {
	return &ProviderModelDiscoveryService{
		http: &http.Client{Timeout: 30 * time.Second},
		keys: keys,
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
		{"alibaba", s.fetchAlibabaModels},
		{"deepseek", s.fetchDeepSeekModels},
		{"mistral", s.fetchMistralModels},
		{"moonshot", s.fetchMoonshotModels},
		{"siliconflow", s.fetchSiliconFlowModels},
		{"zai", s.fetchZAIModels},
		{"xai", s.fetchXAIModels},
		{"poe", s.fetchPoeModels},
		{"fireworks", s.fetchFireworksModels},
		{"together", s.fetchTogetherModels},
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

func readModelsListResponse(resp *http.Response) ([]string, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if len(body) > 0 {
			return nil, fmt.Errorf("status %d body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wrapped struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Data) > 0 {
		models := make([]string, 0, len(wrapped.Data))
		for _, item := range wrapped.Data {
			models = append(models, item.ID)
		}
		return normalizeModelIDs(models), nil
	}
	var direct []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &direct); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(direct))
	for _, item := range direct {
		models = append(models, item.ID)
	}
	return normalizeModelIDs(models), nil
}

func (s *ProviderModelDiscoveryService) fetchOpenAIModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.OpenAI)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
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
	apiKey := strings.TrimSpace(s.keys.Anthropic)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	}
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
	apiKey := strings.TrimSpace(s.keys.Google)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
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
	apiKey := strings.TrimSpace(s.keys.Groq)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	}
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

func (s *ProviderModelDiscoveryService) fetchTogetherModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.Together)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TOGETHER_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := normalizeTogetherAPIBaseURL(os.Getenv("TOGETHER_API_BASE_URL"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	return readModelsListResponse(resp)
}

func normalizeTogetherAPIBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "https://api.together.xyz"
	}
	for _, suffix := range []string{"/v1/chat/completions", "/chat/completions", "/v1"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return base
}

func (s *ProviderModelDiscoveryService) fetchDeepSeekModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.DeepSeek)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	}
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

func (s *ProviderModelDiscoveryService) fetchAlibabaModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.Alibaba)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("ALIBABA_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ALIBABA_API_BASE_URL")), "/")
	if base == "" {
		base = "https://dashscope-us.aliyuncs.com/compatible-mode/v1"
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

func (s *ProviderModelDiscoveryService) fetchMoonshotModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.Moonshot)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("MOONSHOT_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("MOONSHOT_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.moonshot.ai/v1"
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

func (s *ProviderModelDiscoveryService) fetchSiliconFlowModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.SiliconFlow)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("SILICONFLOW_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SILICONFLOW_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.siliconflow.com/v1"
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

func (s *ProviderModelDiscoveryService) fetchPoeModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.Poe)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("POE_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, poeModelsURL(), nil)
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

func (s *ProviderModelDiscoveryService) fetchZAIModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.ZAI)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("ZAI_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ZAI_API_BASE_URL")), "/")
	if base == "" {
		base = "https://api.z.ai/api/paas/v4"
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

func (s *ProviderModelDiscoveryService) fetchMistralModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.Mistral)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("MISTRAL_API_KEY"))
	}
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
	apiKey := strings.TrimSpace(s.keys.XAI)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("XAI_API_KEY"))
	}
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
	Public                bool   `json:"public"`
	BaseModelDetails      struct {
		ModelType string `json:"modelType"`
	} `json:"baseModelDetails"`
	ModelType string `json:"modelType"`
}

func fireworksModelID(name string) string {
	name = strings.TrimSpace(name)
	const marker = "/models/"
	if idx := strings.Index(name, marker); idx >= 0 {
		return strings.TrimSpace(name[idx+len(marker):])
	}
	return name
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

func fireworksSupportsServerless(item fireworksModelListItem) bool {
	if item.SupportsServerless || item.SupportsServerlessAlt {
		return true
	}
	// Fireworks list-models responses do not always include explicit serverless flags.
	// Treat public text models as eligible so discovery does not collapse to zero.
	return item.Public
}

func (s *ProviderModelDiscoveryService) fetchFireworksModels(ctx context.Context) ([]string, error) {
	apiKey := strings.TrimSpace(s.keys.Fireworks)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("FIREWORKS_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	base := strings.TrimSpace(os.Getenv("FIREWORKS_API_BASE_URL"))
	if base == "" {
		base = "https://api.fireworks.ai"
	}
	parsedBase, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if parsedBase.Scheme == "" {
		parsedBase.Scheme = "https"
	}
	if parsedBase.Host == "" {
		parsedBase.Host = parsedBase.Path
		parsedBase.Path = ""
	}
	parsedBase.Path = "/v1/accounts/fireworks/models"
	query := url.Values{}
	query.Set("pageSize", "200")
	models := make([]string, 0, 128)
	for {
		parsedBase.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedBase.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		var decoded struct {
			Data          []fireworksModelListItem `json:"data"`
			Models        []fireworksModelListItem `json:"models"`
			NextPageToken string                   `json:"nextPageToken"`
		}
		resp, err := s.http.Do(req)
		if err != nil {
			return nil, err
		}
		if err := readJSONResponse(resp, &decoded); err != nil {
			return nil, err
		}
		items := decoded.Models
		if len(items) == 0 {
			items = decoded.Data
		}
		for _, item := range items {
			if !fireworksSupportsServerless(item) {
				continue
			}
			if !isFireworksTextModel(item) {
				continue
			}
			models = append(models, fireworksModelID(item.Name))
		}
		if strings.TrimSpace(decoded.NextPageToken) == "" {
			break
		}
		query.Set("pageToken", strings.TrimSpace(decoded.NextPageToken))
	}
	return normalizeModelIDs(models), nil
}
