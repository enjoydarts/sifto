package service

import "testing"

func TestNormalizeCatalogPricedUsagePrefersOpenRouterAliasedPricing(t *testing.T) {
	original := dynamicChatModels
	SetDynamicChatModels([]LLMModelCatalog{
		{
			ID:       OpenRouterAliasModelID("openai/gpt-oss-120b"),
			Provider: "openrouter",
			Pricing: &LLMModelPricing{
				PricingSource:       "openrouter_snapshot",
				InputPerMTokUSD:     2.0,
				OutputPerMTokUSD:    4.0,
				CacheReadPerMTokUSD: 1.0,
			},
		},
	})
	defer SetDynamicChatModels(original)

	usage := &LLMUsage{
		Provider:             "openrouter",
		Model:                OpenRouterAliasModelID("openai/gpt-oss-120b"),
		PricingModelFamily:   "openai/gpt-oss-120b",
		PricingSource:        "worker_estimate",
		InputTokens:          1_000_000,
		OutputTokens:         500_000,
		CacheReadInputTokens: 100_000,
	}

	got := NormalizeCatalogPricedUsage("summary", usage)
	if got == nil {
		t.Fatal("NormalizeCatalogPricedUsage returned nil")
	}
	if got.Provider != "openrouter" {
		t.Fatalf("provider = %q, want openrouter", got.Provider)
	}
	if got.Model != OpenRouterAliasModelID("openai/gpt-oss-120b") {
		t.Fatalf("model = %q", got.Model)
	}
	if got.PricingModelFamily != OpenRouterAliasModelID("openai/gpt-oss-120b") {
		t.Fatalf("pricing_model_family = %q", got.PricingModelFamily)
	}
	if got.PricingSource != "openrouter_snapshot" {
		t.Fatalf("pricing_source = %q, want openrouter_snapshot", got.PricingSource)
	}
	want := 3.9
	if got.EstimatedCostUSD != want {
		t.Fatalf("estimated_cost_usd = %.4f, want %.4f", got.EstimatedCostUSD, want)
	}
}

func TestNormalizeCatalogPricedUsagePrefersResolvedModelForOpenRouterAuto(t *testing.T) {
	original := dynamicChatModels
	SetDynamicChatModels([]LLMModelCatalog{
		{
			ID:       OpenRouterAliasModelID("openai/gpt-oss-120b"),
			Provider: "openrouter",
			Pricing: &LLMModelPricing{
				PricingSource:       "openrouter_snapshot",
				InputPerMTokUSD:     2.0,
				OutputPerMTokUSD:    4.0,
				CacheReadPerMTokUSD: 1.0,
			},
		},
	})
	defer SetDynamicChatModels(original)

	usage := &LLMUsage{
		Provider:             "openrouter",
		Model:                OpenRouterAliasModelID("auto"),
		RequestedModel:       OpenRouterAliasModelID("auto"),
		ResolvedModel:        "openai/gpt-oss-120b",
		PricingModelFamily:   "openai/gpt-oss-120b",
		PricingSource:        "worker_estimate",
		InputTokens:          1_000_000,
		OutputTokens:         500_000,
		CacheReadInputTokens: 100_000,
	}

	got := NormalizeCatalogPricedUsage("summary", usage)
	if got == nil {
		t.Fatal("NormalizeCatalogPricedUsage returned nil")
	}
	if got.Model != OpenRouterAliasModelID("auto") {
		t.Fatalf("model = %q", got.Model)
	}
	if got.PricingModelFamily != OpenRouterAliasModelID("openai/gpt-oss-120b") {
		t.Fatalf("pricing_model_family = %q", got.PricingModelFamily)
	}
	if got.PricingSource != "openrouter_snapshot" {
		t.Fatalf("pricing_source = %q, want openrouter_snapshot", got.PricingSource)
	}
	want := 3.9
	if got.EstimatedCostUSD != want {
		t.Fatalf("estimated_cost_usd = %.4f, want %.4f", got.EstimatedCostUSD, want)
	}
}

func TestNormalizeCatalogPricedUsageFallsBackToRequestedModelWhenResolvedModelMissing(t *testing.T) {
	original := dynamicChatModels
	SetDynamicChatModels([]LLMModelCatalog{
		{
			ID:       OpenRouterAliasModelID("qwen/qwen3.5-flash-02-23"),
			Provider: "openrouter",
			Pricing: &LLMModelPricing{
				PricingSource:    "openrouter_snapshot",
				InputPerMTokUSD:  1.5,
				OutputPerMTokUSD: 3.0,
			},
		},
	})
	defer SetDynamicChatModels(original)

	usage := &LLMUsage{
		Provider:       "openrouter",
		Model:          OpenRouterAliasModelID("qwen/qwen3.5-flash-02-23"),
		RequestedModel: OpenRouterAliasModelID("qwen/qwen3.5-flash-02-23"),
		ResolvedModel:  "qwen/qwen3.5-flash-20260224",
		PricingSource:  "worker_estimate",
		InputTokens:    1_000_000,
		OutputTokens:   500_000,
	}

	got := NormalizeCatalogPricedUsage("summary", usage)
	if got == nil {
		t.Fatal("NormalizeCatalogPricedUsage returned nil")
	}
	if got.PricingModelFamily != OpenRouterAliasModelID("qwen/qwen3.5-flash-02-23") {
		t.Fatalf("pricing_model_family = %q", got.PricingModelFamily)
	}
	if got.PricingSource != "openrouter_snapshot" {
		t.Fatalf("pricing_source = %q, want openrouter_snapshot", got.PricingSource)
	}
	want := 3.0
	if got.EstimatedCostUSD != want {
		t.Fatalf("estimated_cost_usd = %.4f, want %.4f", got.EstimatedCostUSD, want)
	}
}
