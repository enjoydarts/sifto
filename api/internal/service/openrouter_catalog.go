package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type OpenRouterCatalogService struct {
	http *http.Client
}

func NewOpenRouterCatalogService() *OpenRouterCatalogService {
	return &OpenRouterCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type openRouterModelsResponse struct {
	Data []openRouterModel `json:"data"`
}

type openRouterModel struct {
	ID                  string         `json:"id"`
	CanonicalSlug       string         `json:"canonical_slug"`
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	ContextLength       int            `json:"context_length"`
	Pricing             map[string]any `json:"pricing"`
	SupportedParameters []string       `json:"supported_parameters"`
	Architecture        map[string]any `json:"architecture"`
	TopProvider         map[string]any `json:"top_provider"`
	Modalities          []string       `json:"modalities"`
}

func (s *OpenRouterCatalogService) FetchTextGenerationModels(ctx context.Context) ([]repository.OpenRouterModelSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterModelsURL(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openrouter models api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload openRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]repository.OpenRouterModelSnapshot, 0, len(payload.Data))
	now := time.Now().UTC()
	for _, item := range payload.Data {
		if !isOpenRouterTextModel(item) {
			continue
		}
		out = append(out, normalizeOpenRouterModel(item, now))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderSlug == out[j].ProviderSlug {
			return out[i].DisplayName < out[j].DisplayName
		}
		return out[i].ProviderSlug < out[j].ProviderSlug
	})
	return out, nil
}

func OpenRouterSnapshotsToCatalogModels(models []repository.OpenRouterModelSnapshot) []LLMModelCatalog {
	out := make([]LLMModelCatalog, 0, len(models))
	for _, item := range models {
		comment := strings.TrimSpace(derefOptionalString(item.DescriptionJA))
		if comment == "" {
			comment = strings.TrimSpace(derefOptionalString(item.DescriptionEN))
		}
		if comment == "" {
			comment = "OpenRouter 経由で利用できる実験モデル。"
		}
		supportedParams := parseJSONArray(item.SupportedParametersJSON)
		out = append(out, LLMModelCatalog{
			ID:                OpenRouterAliasModelID(item.ModelID),
			Provider:          "openrouter",
			SourceProvider:    item.ProviderSlug,
			AvailablePurposes: []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"},
			Recommendation:    "strong",
			BestFor:           "experimental",
			Highlights:        []string{"experimental"},
			Comment:           comment,
			Capabilities: &LLMModelCapabilities{
				SupportsStructuredOutput:  supportsAnyParam(supportedParams, "response_format", "structured_outputs"),
				SupportsStrictJSONSchema:  false,
				SupportsReasoning:         supportsAnyParam(supportedParams, "reasoning"),
				SupportsToolCalling:       supportsAnyParam(supportedParams, "tools"),
				SupportsCacheReadPricing:  false,
				SupportsCacheWritePricing: false,
			},
			Pricing: &LLMModelPricing{
				PricingSource:       "openrouter_snapshot",
				InputPerMTokUSD:     parseOpenRouterPrice(item.PricingJSON, "prompt"),
				OutputPerMTokUSD:    parseOpenRouterPrice(item.PricingJSON, "completion"),
				CacheReadPerMTokUSD: parseOpenRouterPrice(item.PricingJSON, "cache_read"),
			},
		})
	}
	return out
}

func EnrichOpenRouterDescriptionsJA(ctx context.Context, repo *repository.OpenRouterModelRepo, openAI *OpenAIClient, models []repository.OpenRouterModelSnapshot) []repository.OpenRouterModelSnapshot {
	if len(models) == 0 {
		return models
	}
	cache := map[string]repository.OpenRouterDescriptionCacheEntry{}
	if repo != nil {
		if cached, err := repo.ListLatestDescriptionCache(ctx); err == nil {
			cache = cached
		}
	}
	missing := make(map[string]string)
	for i := range models {
		descEN := strings.TrimSpace(derefOptionalString(models[i].DescriptionEN))
		if descEN == "" {
			continue
		}
		if cached, ok := cache[models[i].ModelID]; ok && strings.TrimSpace(derefOptionalString(cached.DescriptionEN)) == descEN {
			if translated := strings.TrimSpace(derefOptionalString(cached.DescriptionJA)); translated != "" && translated != descEN {
				models[i].DescriptionJA = &translated
				continue
			}
		}
		missing[models[i].ModelID] = descEN
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if openAI == nil || apiKey == "" || len(missing) == 0 {
		return models
	}
	ids := make([]string, 0, len(missing))
	for modelID := range missing {
		ids = append(ids, modelID)
	}
	sort.Strings(ids)
	for _, modelID := range ids {
		translated, err := openAI.TranslateTextsToJA(ctx, apiKey, OpenRouterDescriptionTranslationModel(), map[string]string{
			modelID: missing[modelID],
		})
		if err != nil {
			continue
		}
		for i := range models {
			if text, ok := translated[models[i].ModelID]; ok && strings.TrimSpace(text) != "" {
				trimmed := strings.TrimSpace(text)
				models[i].DescriptionJA = &trimmed
			}
		}
	}
	return models
}

func openRouterModelsURL() string {
	if v := strings.TrimSpace(os.Getenv("OPENROUTER_MODELS_API_URL")); v != "" {
		return v
	}
	return "https://openrouter.ai/api/v1/models"
}

func isOpenRouterTextModel(item openRouterModel) bool {
	lowerID := strings.ToLower(strings.TrimSpace(item.ID))
	if lowerID == "" {
		return false
	}
	blocked := []string{"embed", "embedding", "moderation", "rerank", "reranker", "tts", "transcription", "speech"}
	for _, token := range blocked {
		if strings.Contains(lowerID, token) {
			return false
		}
	}
	return true
}

func normalizeOpenRouterModel(item openRouterModel, fetchedAt time.Time) repository.OpenRouterModelSnapshot {
	canonical := strings.TrimSpace(item.CanonicalSlug)
	if canonical == "" {
		canonical = strings.TrimSpace(item.ID)
	}
	providerSlug := openRouterProviderSlug(item.ID)
	descEN := trimPtr(item.Description)
	var contextLength *int
	if item.ContextLength > 0 {
		contextLength = &item.ContextLength
	}
	return repository.OpenRouterModelSnapshot{
		ModelID:                 strings.TrimSpace(item.ID),
		CanonicalSlug:           trimPtr(canonical),
		ProviderSlug:            providerSlug,
		DisplayName:             strings.TrimSpace(item.Name),
		DescriptionEN:           descEN,
		DescriptionJA:           nil,
		ContextLength:           contextLength,
		PricingJSON:             mustJSON(item.Pricing, "{}"),
		SupportedParametersJSON: mustJSON(item.SupportedParameters, "[]"),
		ArchitectureJSON:        mustJSON(item.Architecture, "{}"),
		TopProviderJSON:         mustJSON(item.TopProvider, "{}"),
		ModalityFlagsJSON:       mustJSON(map[string]any{"modalities": item.Modalities}, "{}"),
		IsTextGeneration:        true,
		IsActive:                true,
		FetchedAt:               fetchedAt,
	}
}

func openRouterProviderSlug(modelID string) string {
	parts := strings.Split(strings.TrimSpace(modelID), "/")
	if len(parts) > 1 && strings.TrimSpace(parts[0]) != "" {
		return strings.TrimSpace(parts[0])
	}
	return "other"
}

func mustJSON(v any, fallback string) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(fallback)
	}
	return raw
}

func trimPtr(v string) *string {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return &s
}

func derefOptionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func parseJSONArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func supportsAnyParam(values []string, wants ...string) bool {
	for _, value := range values {
		for _, want := range wants {
			if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
				return true
			}
		}
	}
	return false
}

func parseOpenRouterPrice(raw json.RawMessage, key string) float64 {
	if len(raw) == 0 {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	v, ok := payload[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t * 1_000_000
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return f * 1_000_000
		}
	}
	return 0
}
