package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func testStringPtr(v string) *string { return &v }

func TestBuildOpenRouterBackfillUpdateRecomputesCanonicalCost(t *testing.T) {
	original := service.LLMCatalogData()
	_ = original
	prevDynamic := []service.LLMModelCatalog{
		{
			ID:       service.OpenRouterAliasModelID("anthropic/claude-opus-4.6"),
			Provider: "openrouter",
			Pricing: &service.LLMModelPricing{
				PricingSource:    "openrouter_snapshot",
				InputPerMTokUSD:  5.0,
				OutputPerMTokUSD: 25.0,
			},
		},
	}
	service.SetDynamicChatModels(prevDynamic)
	defer service.SetDynamicChatModels(nil)

	row := repository.LLMUsageLog{
		ID:             "u1",
		Provider:       "openrouter",
		Purpose:        "summary",
		Model:          service.OpenRouterAliasModelID("auto"),
		RequestedModel: testStringPtr("openrouter::auto"),
		ResolvedModel:  testStringPtr("anthropic/claude-4.6-opus-20260205"),
		InputTokens:    1_000_000,
		OutputTokens:   500_000,
	}

	got := buildOpenRouterBackfillUpdate(row)
	if got.Zeroed {
		t.Fatal("resolved canonical model should not be zeroed")
	}
	if got.PricingSource != "openrouter_snapshot" {
		t.Fatalf("pricing_source = %q", got.PricingSource)
	}
	if got.PricingModelFamily == nil || *got.PricingModelFamily != service.OpenRouterAliasModelID("anthropic/claude-opus-4.6") {
		t.Fatalf("pricing_model_family = %#v", got.PricingModelFamily)
	}
	if got.EstimatedCostUSD != 17.5 {
		t.Fatalf("estimated_cost_usd = %.4f", got.EstimatedCostUSD)
	}
}

func TestBuildOpenRouterBackfillUpdateZeroesUnresolvedRows(t *testing.T) {
	row := repository.LLMUsageLog{
		ID:               "u2",
		Provider:         "openrouter",
		Purpose:          "summary",
		Model:            service.OpenRouterAliasModelID("auto"),
		RequestedModel:   testStringPtr("openrouter::auto"),
		ResolvedModel:    testStringPtr("unknown/provider-model-20260205"),
		EstimatedCostUSD: -1000000,
		InputTokens:      1_000_000,
		OutputTokens:     500_000,
	}

	got := buildOpenRouterBackfillUpdate(row)
	if !got.Zeroed {
		t.Fatal("unresolved row should be zeroed")
	}
	if got.PricingSource != "openrouter_backfill_zeroed" {
		t.Fatalf("pricing_source = %q", got.PricingSource)
	}
	if got.EstimatedCostUSD != 0 {
		t.Fatalf("estimated_cost_usd = %.4f, want 0", got.EstimatedCostUSD)
	}
}
