package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type PoeCatalogService struct {
	http *http.Client
}

func NewPoeCatalogService() *PoeCatalogService {
	return &PoeCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type poeModelsResponse struct {
	Data []poeModel `json:"data"`
}

type poeModel struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	OwnedBy       string         `json:"owned_by"`
	ContextLength int            `json:"context_length"`
	Pricing       map[string]any `json:"pricing"`
	Architecture  map[string]any `json:"architecture"`
	Modalities    []string       `json:"modalities"`
}

func (s *PoeCatalogService) FetchModels(ctx context.Context, apiKey string) ([]repository.PoeModelSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, poeModelsURL(), nil)
	if err != nil {
		return nil, err
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("poe models api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload poeModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]repository.PoeModelSnapshot, 0, len(payload.Data))
	now := time.Now().UTC()
	for _, item := range payload.Data {
		out = append(out, normalizePoeModel(item, now))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OwnedBy == out[j].OwnedBy {
			return out[i].DisplayName < out[j].DisplayName
		}
		return out[i].OwnedBy < out[j].OwnedBy
	})
	return out, nil
}

func PoeSupportsAnthropicCompat(model repository.PoeModelSnapshot) bool {
	id := strings.ToLower(strings.TrimSpace(model.ModelID))
	ownedBy := strings.ToLower(strings.TrimSpace(model.OwnedBy))
	return strings.Contains(id, "claude") && ownedBy == "anthropic"
}

func PoePreferredTransport(model repository.PoeModelSnapshot) string {
	if PoeSupportsAnthropicCompat(model) {
		return "anthropic"
	}
	return "openai"
}

func PoeSnapshotsToCatalogModels(models []repository.PoeModelSnapshot) []LLMModelCatalog {
	out := make([]LLMModelCatalog, 0, len(models))
	for _, item := range models {
		comment := strings.TrimSpace(derefPoeOptionalString(item.DescriptionJA))
		if comment == "" {
			comment = strings.TrimSpace(derefPoeOptionalString(item.DescriptionEN))
		}
		if comment == "" {
			comment = "Poe 経由で利用できるモデル。"
		}
		out = append(out, LLMModelCatalog{
			ID:                PoeAliasModelID(item.ModelID),
			Provider:          "poe",
			SourceProvider:    item.OwnedBy,
			AvailablePurposes: []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"},
			Recommendation:    "strong",
			BestFor:           "experimental",
			Highlights:        []string{"experimental"},
			Comment:           comment,
			Capabilities: &LLMModelCapabilities{
				SupportsStructuredOutput:  false,
				SupportsStrictJSONSchema:  false,
				SupportsReasoning:         false,
				SupportsToolCalling:       false,
				SupportsCacheReadPricing:  false,
				SupportsCacheWritePricing: false,
			},
			Pricing: &LLMModelPricing{
				PricingSource:       "poe_snapshot",
				InputPerMTokUSD:     parsePoePrice(item.PricingJSON, "prompt"),
				OutputPerMTokUSD:    parsePoePrice(item.PricingJSON, "completion"),
				CacheReadPerMTokUSD: parsePoePrice(item.PricingJSON, "cache_read"),
			},
		})
	}
	return out
}

func derefPoeOptionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func normalizePoeModel(item poeModel, fetchedAt time.Time) repository.PoeModelSnapshot {
	modelID := strings.TrimSpace(item.ID)
	displayName := strings.TrimSpace(item.Name)
	if displayName == "" {
		displayName = modelID
	}
	description := strings.TrimSpace(item.Description)
	var descriptionPtr *string
	if description != "" {
		descriptionPtr = &description
	}
	var contextLength *int
	if item.ContextLength > 0 {
		contextLength = &item.ContextLength
	}
	pricingJSON, _ := json.Marshal(item.Pricing)
	architectureJSON, _ := json.Marshal(item.Architecture)
	modalityFlagsJSON, _ := json.Marshal(map[string]any{"modalities": item.Modalities})
	snapshot := repository.PoeModelSnapshot{
		ModelID:                        modelID,
		DisplayName:                    displayName,
		OwnedBy:                        strings.TrimSpace(item.OwnedBy),
		DescriptionEN:                  descriptionPtr,
		ContextLength:                  contextLength,
		PricingJSON:                    pricingJSON,
		ArchitectureJSON:               architectureJSON,
		ModalityFlagsJSON:              modalityFlagsJSON,
		IsActive:                       true,
		TransportSupportsOpenAICompat:  true,
		TransportSupportsAnthropicCompat: false,
		FetchedAt:                      fetchedAt,
	}
	snapshot.TransportSupportsAnthropicCompat = PoeSupportsAnthropicCompat(snapshot)
	snapshot.PreferredTransport = PoePreferredTransport(snapshot)
	return snapshot
}

func poeModelsURL() string {
	return "https://api.poe.com/v1/models"
}

func parsePoePrice(raw json.RawMessage, key string) float64 {
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
