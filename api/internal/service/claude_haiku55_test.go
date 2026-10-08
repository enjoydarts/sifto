package service

import (
	"encoding/json"
	"testing"
)

func TestClaudeHaiku55Catalog(t *testing.T) {
	const model = "claude-haiku-5-5"
	entry := CatalogModelByID(model)
	if entry == nil || entry.Pricing == nil {
		t.Fatal("Haiku 5.5 must be in the model catalog")
	}
	for _, purpose := range []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"} {
		if !CatalogModelSupportsPurpose(model, purpose) {
			t.Errorf("Haiku 5.5 is unavailable for %s", purpose)
		}
	}
	if CatalogProviderForModel(model) != "anthropic" || !CatalogModelSupportsCapability(model, "structured_output") {
		t.Fatal("Haiku 5.5 must use Anthropic and support JSON tasks")
	}
	if entry.Pricing.InputPerMTokUSD != 0.1 || entry.Pricing.OutputPerMTokUSD != 0.5 || entry.Pricing.CacheWritePerMTokUSD != 0.125 || entry.Pricing.CacheReadPerMTokUSD != 0.01 {
		t.Fatalf("unexpected base pricing: %+v", entry.Pricing)
	}
}

func TestClaudeHaiku55UsagePricing(t *testing.T) {
	for _, tc := range []struct {
		name               string
		input, write, read int
		cost               float64
	}{
		{"at boundary", 100_000, 0, 0, 0.0105},
		{"above boundary", 100_001, 0, 0, 0.0525005},
		{"cache read crosses boundary", 1, 0, 100_000, 0.0075005},
		{"cache write crosses boundary", 1, 100_000, 0, 0.0650005},
		{"cache input categories separate", 500, 200, 300, 0.000578},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usage := NormalizeCatalogPricedUsage("summary", &LLMUsage{
				Provider: "anthropic", Model: "claude-haiku-5-5", InputTokens: tc.input,
				OutputTokens: 1000, CacheCreationInputTokens: tc.write, CacheReadInputTokens: tc.read,
			})
			if usage.EstimatedCostUSD != tc.cost || usage.PricingSource != "anthropic_docs_2026_10" {
				t.Fatalf("unexpected pricing: %+v, want %v", usage, tc.cost)
			}
		})
	}
}

func TestClaudeHaiku55MergedUsagePreservesPerCallCost(t *testing.T) {
	// A 60K request and a 160K request pay different rates before their usage is merged.
	var usage LLMUsage
	if err := json.Unmarshal([]byte(`{"provider":"anthropic","model":"claude-haiku-5-5","pricing_model_family":"claude-haiku-5-5","pricing_source":"anthropic_docs_2026_10","input_tokens":220000,"output_tokens":2000,"estimated_cost_usd":0.089,"calls":2}`), &usage); err != nil {
		t.Fatal(err)
	}
	got := NormalizeCatalogPricedUsage("facts", &usage)
	if got.EstimatedCostUSD != 0.089 || got.PricingSource != "anthropic_docs_2026_10" {
		t.Fatalf("merged usage must preserve per-call cost: %+v", got)
	}
}

func TestClaudeHaiku55MergedShortRequestsKeepLowerRate(t *testing.T) {
	var usage LLMUsage
	if err := json.Unmarshal([]byte(`{"provider":"anthropic","model":"claude-haiku-5-5","pricing_source":"anthropic_docs_2026_10","input_tokens":120000,"output_tokens":2000,"estimated_cost_usd":0.013,"calls":2}`), &usage); err != nil {
		t.Fatal(err)
	}
	if got := NormalizeCatalogPricedUsage("facts", &usage); got.EstimatedCostUSD != 0.013 {
		t.Fatalf("two 60K requests must keep lower rate: %+v", got)
	}
}
