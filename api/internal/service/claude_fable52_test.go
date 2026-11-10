package service

import "testing"

func TestClaudeFable52CatalogAndUsagePricing(t *testing.T) {
	const model = "claude-fable-5-2"
	entry := CatalogModelByID(model)
	if entry == nil || entry.Pricing == nil {
		t.Fatal("Fable 5.2 must be in the model catalog")
	}
	for _, purpose := range []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"} {
		if !CatalogModelSupportsPurpose(model, purpose) {
			t.Errorf("Fable 5.2 is unavailable for %s", purpose)
		}
	}
	if CatalogProviderForModel(model) != "anthropic" || !CatalogModelSupportsCapability(model, "structured_output") {
		t.Fatal("Fable 5.2 must route JSON tasks to Anthropic")
	}
	if entry.Pricing.InputPerMTokUSD != 8 || entry.Pricing.OutputPerMTokUSD != 40 || entry.Pricing.CacheWritePerMTokUSD != 10 || entry.Pricing.CacheReadPerMTokUSD != 0.2 {
		t.Fatalf("unexpected pricing: %+v", entry.Pricing)
	}
	usage := NormalizeCatalogPricedUsage("facts", &LLMUsage{
		Provider: "anthropic", Model: model, InputTokens: 500, OutputTokens: 1000,
		CacheCreationInputTokens: 200, CacheReadInputTokens: 300,
	})
	if usage.EstimatedCostUSD != 0.04606 || usage.PricingSource != "anthropic_docs_2026_11" {
		t.Fatalf("unexpected normalized usage: %+v", usage)
	}
}
