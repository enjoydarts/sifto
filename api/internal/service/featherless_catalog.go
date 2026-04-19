package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

type FeatherlessCatalogService struct {
	http *http.Client
}

func NewFeatherlessCatalogService() *FeatherlessCatalogService {
	return &FeatherlessCatalogService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *FeatherlessCatalogService) FetchModels(ctx context.Context, apiKey string) ([]repository.FeatherlessModelSnapshot, error) {
	discovery := NewProviderModelDiscoveryServiceWithKeys(ProviderModelDiscoveryKeys{
		Featherless: strings.TrimSpace(apiKey),
	})
	discovery.http = s.http
	return discovery.fetchFeatherlessSnapshots(ctx)
}

func FeatherlessSnapshotsToCatalogModels(models []repository.FeatherlessModelSnapshot) []LLMModelCatalog {
	out := make([]LLMModelCatalog, 0, len(models))
	seen := map[string]struct{}{}
	for _, item := range models {
		resolved := strings.TrimSpace(ResolveFeatherlessModelID(item.ModelID))
		if resolved == "" {
			continue
		}
		aliased := FeatherlessAliasModelID(resolved)
		if _, ok := seen[aliased]; ok {
			continue
		}
		seen[aliased] = struct{}{}
		sourceProvider := strings.TrimSpace(strings.SplitN(resolved, "/", 2)[0])
		comment := "Featherless.ai の dynamic snapshot から取り込んだ OpenAI 互換モデル。価格は snapshot 未連携のため 0 扱い。"
		if item.ModelClass != "" {
			comment = "Featherless.ai の dynamic snapshot から取り込んだ " + item.ModelClass + " モデル。価格は snapshot 未連携のため 0 扱い。"
		}
		out = append(out, LLMModelCatalog{
			ID:                aliased,
			Provider:          "featherless",
			SourceProvider:    sourceProvider,
			AvailablePurposes: []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"},
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
				PricingSource:       "featherless_snapshot",
				InputPerMTokUSD:     0,
				OutputPerMTokUSD:    0,
				CacheReadPerMTokUSD: 0,
			},
		})
	}
	return out
}

func FeatherlessModelIDsToCatalogModels(models []string) []LLMModelCatalog {
	snapshots := make([]repository.FeatherlessModelSnapshot, 0, len(models))
	for _, modelID := range models {
		resolved := strings.TrimSpace(ResolveFeatherlessModelID(modelID))
		if resolved == "" {
			continue
		}
		snapshots = append(snapshots, repository.FeatherlessModelSnapshot{
			ModelID:                resolved,
			DisplayName:            resolved,
			AvailableOnCurrentPlan: true,
		})
	}
	return FeatherlessSnapshotsToCatalogModels(snapshots)
}
