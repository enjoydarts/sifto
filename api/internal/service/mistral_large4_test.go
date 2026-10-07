package service

import "testing"

func TestMistralLarge4Catalog(t *testing.T) {
	const model = "mistral-large-4"
	entry := CatalogModelByID(model)
	if entry == nil || CatalogProviderForModel(model) != "mistral" {
		t.Fatal("Mistral Large 4 must resolve to the Mistral catalog")
	}
	if CatalogProviderForModel("mistral-large-4-0") != "mistral" {
		t.Fatal("the official Mistral Large 4 alias must use the Mistral provider")
	}
	for _, purpose := range []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"} {
		if !CatalogModelSupportsPurpose(model, purpose) {
			t.Errorf("Mistral Large 4 is unavailable for %s", purpose)
		}
	}
	if !CatalogModelSupportsCapability(model, "structured_output") || !CatalogModelSupportsCapability(model, "cache_read_pricing") {
		t.Fatal("structured output and cached input pricing must be supported")
	}
	if entry.Pricing == nil || entry.Pricing.InputPerMTokUSD != 1.36 || entry.Pricing.OutputPerMTokUSD != 4.18 || entry.Pricing.CacheReadPerMTokUSD != 0.14 {
		t.Fatalf("unexpected standard pricing: %+v", entry.Pricing)
	}
}

func TestMistralLarge4UsageCost(t *testing.T) {
	usage := NormalizeCatalogPricedUsage("facts", &LLMUsage{
		Provider:             "mistral",
		Model:                "mistral-large-4",
		InputTokens:          1000,
		OutputTokens:         200,
		CacheReadInputTokens: 400,
	})
	if usage.PricingSource != "mistral_docs_2026_10_standard" || usage.EstimatedCostUSD != 0.001708 {
		t.Fatalf("unexpected usage pricing: %+v", usage)
	}
}
