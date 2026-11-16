package service

import (
	"testing"
	"time"
)

func TestGemini4Catalog(t *testing.T) {
	for _, model := range []string{"gemini-4-carbon", "gemini-4-argon", "gemini-4-hydrogen"} {
		t.Run(model, func(t *testing.T) {
			entry := CatalogModelByID(model)
			if entry == nil || entry.Pricing == nil {
				t.Fatal("Gemini 4 model missing from catalog")
			}
			if CatalogProviderForModel(model) != "google" {
				t.Fatal("wrong provider")
			}
			for _, purpose := range []string{"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"} {
				if !CatalogModelSupportsPurpose(model, purpose) {
					t.Errorf("unavailable for %s", purpose)
				}
			}
		})
	}
}

func TestGemini4PricingChange(t *testing.T) {
	for _, model := range []string{"gemini-4-carbon", "gemini-4-argon"} {
		entry := CatalogModelByID(model)
		if entry == nil {
			t.Fatalf("missing %s", model)
		}
		// Resolve from the immutable source rates, not a previously resolved view.
		base := *entry.Pricing
		base.InputPerMTokUSD, base.OutputPerMTokUSD, base.CacheReadPerMTokUSD = 2, 10, 0.1
		for _, tc := range []struct {
			at                    string
			input, output, cached float64
		}{
			{"2026-11-30T23:59:59Z", 2, 10, 0.1}, {"2026-12-01T00:00:00Z", 4, 20, 0.2},
		} {
			at, _ := time.Parse(time.RFC3339, tc.at)
			got := catalogPricingAt(&base, at)
			if got.InputPerMTokUSD != tc.input || got.OutputPerMTokUSD != tc.output || got.CacheReadPerMTokUSD != tc.cached {
				t.Fatalf("%s at %s: %+v", model, tc.at, got)
			}
		}
		if base.InputPerMTokUSD != 2 {
			t.Fatal("changed cached source pricing")
		}
		usage := NormalizeCatalogPricedUsage("summary", &LLMUsage{Provider: "google", Model: model, Calls: 2, EstimatedCostUSD: 0.123, PricingSource: "per_request"})
		if usage.EstimatedCostUSD != 0.123 || usage.PricingSource != "per_request" {
			t.Fatal("merged request costs were re-priced across a date boundary")
		}
	}
}

func TestLatestAnthropicCatalogPrices(t *testing.T) {
	for _, tc := range []struct {
		model                          string
		input, output, written, cached float64
	}{
		{"claude-fable-5-2", 8, 40, 10, 0.2}, {"claude-opus-5-5", 4, 20, 5, 0.2},
		{"claude-sonnet-5-5", 2, 10, 2.5, 0.1}, {"claude-haiku-5-5", 0.1, 0.5, 0.125, 0.01},
	} {
		entry := CatalogModelByID(tc.model)
		if entry == nil || entry.Pricing == nil {
			t.Fatalf("missing %s", tc.model)
		}
		p := entry.Pricing
		if p.InputPerMTokUSD != tc.input || p.OutputPerMTokUSD != tc.output || p.CacheWritePerMTokUSD != tc.written || p.CacheReadPerMTokUSD != tc.cached {
			t.Fatalf("incorrect current prices for %s: %+v", tc.model, p)
		}
	}
}
