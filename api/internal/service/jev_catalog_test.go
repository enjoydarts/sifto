package service

import "testing"

func TestJevCatalogResolvesConcreteModelBeforeAliasAndPrefix(t *testing.T) {
	catalog := JevCatalog{
		DefaultModel: "jev-latest",
		Models: []JevModelConfig{
			{ID: "jev-2", MatchExact: []string{"jev-2-20261001"}, InputPerMTokUSD: 0.08, PricingSource: "jev_2"},
			{ID: "jev-latest", MatchExact: []string{"jev-latest"}, MatchPrefix: []string{"jev-"}, InputPerMTokUSD: 0.042, PricingSource: "jev_1"},
		},
	}

	got, ok := catalog.ResolvePricing("jev-2-20261001", "jev-latest")
	if !ok || got.PricingSource != "jev_2" || got.InputPerMTokUSD != 0.08 {
		t.Fatalf("ResolvePricing() = %#v, %v", got, ok)
	}
}

func TestJevCatalogFallsBackToRequestedAliasThenDefault(t *testing.T) {
	catalog := JevCatalog{
		DefaultModel: "jev-latest",
		Models: []JevModelConfig{
			{ID: "jev-latest", MatchExact: []string{"jev-latest"}, InputPerMTokUSD: 0.042, PricingSource: "jev_1"},
		},
	}

	got, ok := catalog.ResolvePricing("unknown-concrete", "jev-latest")
	if !ok || got.PricingSource != "jev_1" {
		t.Fatalf("alias ResolvePricing() = %#v, %v", got, ok)
	}
	got, ok = catalog.ResolvePricing("unknown-concrete", "unknown-alias")
	if !ok || got.ID != "jev-latest" {
		t.Fatalf("default ResolvePricing() = %#v, %v", got, ok)
	}
}

func TestLoadJevCatalogIncludesReplaceablePricingAndPolicy(t *testing.T) {
	catalog, err := LoadJevCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.DefaultModel != "jev-latest" {
		t.Fatalf("DefaultModel = %q", catalog.DefaultModel)
	}
	if catalog.GatePolicy.Version != "jev-quality-gate-v1" {
		t.Fatalf("policy version = %q", catalog.GatePolicy.Version)
	}
	pricing, ok := catalog.ResolvePricing("jev-latest", "jev-latest")
	if !ok || pricing.InputPerMTokUSD != 0.042 || pricing.OutputPerMTokUSD != 0 {
		t.Fatalf("pricing = %#v, %v", pricing, ok)
	}
}
