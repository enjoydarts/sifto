package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type DeepInfraCatalogService struct {
	http *http.Client
}

func NewDeepInfraCatalogService() *DeepInfraCatalogService {
	return &DeepInfraCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *DeepInfraCatalogService) FetchModels(ctx context.Context, apiKey string) ([]repository.DeepInfraModelSnapshot, error) {
	discovery := NewProviderModelDiscoveryServiceWithKeys(ProviderModelDiscoveryKeys{
		DeepInfra: strings.TrimSpace(apiKey),
	})
	discovery.http = s.http
	return discovery.fetchDeepInfraSnapshots(ctx)
}

func DeepInfraSnapshotsToCatalogModels(models []repository.DeepInfraModelSnapshot) []LLMModelCatalog {
	out := make([]LLMModelCatalog, 0, len(models))
	seen := map[string]struct{}{}
	for _, item := range models {
		resolved := strings.TrimSpace(ResolveDeepInfraModelID(item.ModelID))
		if resolved == "" {
			continue
		}
		aliased := DeepInfraAliasModelID(resolved)
		if _, ok := seen[aliased]; ok {
			continue
		}
		seen[aliased] = struct{}{}
		sourceProvider := strings.TrimSpace(item.ProviderSlug)
		if sourceProvider == "" {
			sourceProvider = strings.TrimSpace(strings.SplitN(resolved, "/", 2)[0])
		}
		comment := strings.TrimSpace(derefOptionalString(item.DescriptionJA))
		if comment == "" {
			comment = strings.TrimSpace(derefOptionalString(item.DescriptionEN))
		}
		if comment == "" {
			comment = "DeepInfra の dynamic snapshot から取り込んだ OpenAI 互換モデル。"
		}
		if item.ReportedType != "" {
			if strings.TrimSpace(derefOptionalString(item.DescriptionJA)) == "" && strings.TrimSpace(derefOptionalString(item.DescriptionEN)) == "" {
				comment = "DeepInfra の dynamic snapshot から取り込んだ " + item.ReportedType + " モデル。"
			}
		}
		out = append(out, LLMModelCatalog{
			ID:                aliased,
			Provider:          "deepinfra",
			SourceProvider:    sourceProvider,
			AvailablePurposes: []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion", "ai_briefing", "audio_briefing", "faithfulness_check", "facts_check"},
			Recommendation:    "strong",
			BestFor:           "experimental",
			Highlights:        []string{"experimental"},
			Comment:           comment,
			Capabilities: &LLMModelCapabilities{
				SupportsStructuredOutput:  true,
				SupportsStrictJSONSchema:  false,
				SupportsReasoning:         false,
				SupportsToolCalling:       false,
				SupportsCacheReadPricing:  false,
				SupportsCacheWritePricing: false,
			},
			Pricing: &LLMModelPricing{
				PricingSource:       "deepinfra_snapshot",
				InputPerMTokUSD:     derefFloat(item.InputPerMTokUSD),
				OutputPerMTokUSD:    derefFloat(item.OutputPerMTokUSD),
				CacheReadPerMTokUSD: derefFloat(item.CacheReadPerMTokUSD),
			},
		})
	}
	return out
}

func EnrichDeepInfraDescriptionsJA(ctx context.Context, repo *repository.DeepInfraModelRepo, openAI *OpenAIClient, models []repository.DeepInfraModelSnapshot) []repository.DeepInfraModelSnapshot {
	if len(models) == 0 {
		return models
	}
	cache := map[string]repository.DeepInfraDescriptionCacheEntry{}
	if repo != nil {
		if cached, err := repo.ListLatestDescriptionCache(ctx); err == nil {
			cache = cached
		}
	}
	models, missing := ApplyDeepInfraDescriptionCache(models, cache)
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

func ApplyDeepInfraDescriptionCache(
	models []repository.DeepInfraModelSnapshot,
	cache map[string]repository.DeepInfraDescriptionCacheEntry,
) ([]repository.DeepInfraModelSnapshot, map[string]string) {
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
	return models, missing
}

func deepInfraTags(item repository.DeepInfraModelSnapshot) []string {
	if len(item.TagsJSON) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(item.TagsJSON, &tags); err != nil {
		return nil
	}
	return tags
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
