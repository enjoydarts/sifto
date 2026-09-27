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
	if catalog.GatePolicy.Version != "jev-quality-gate-v3" {
		t.Fatalf("policy version = %q", catalog.GatePolicy.Version)
	}
	pricing, ok := catalog.ResolvePricing("jev-latest", "jev-latest")
	if !ok || pricing.InputPerMTokUSD != 0.042 || pricing.OutputPerMTokUSD != 0 {
		t.Fatalf("pricing = %#v, %v", pricing, ok)
	}
}

func TestJevCatalogPolicyAcceptsConsistentMostlyCompliantScores(t *testing.T) {
	catalog, err := LoadJevCatalog()
	if err != nil {
		t.Fatal(err)
	}
	dimensions := map[string]JevDimension{
		"source_support":     {Score: 0.75, RawScore: 3, Confidence: 0.80},
		"contradiction_free": {Score: 0.75, RawScore: 3, Confidence: 0.80},
		"coverage":           {Score: 0.75, RawScore: 3, Confidence: 0.80},
		"inference_control":  {Score: 0.75, RawScore: 3, Confidence: 0.80},
		"specificity":        {Score: 0.75, RawScore: 3, Confidence: 0.80},
	}

	got := EvaluateJevGate(dimensions, []string{"source_support", "contradiction_free"}, catalog.GatePolicy)
	if got.Decision != JevDecisionAccepted {
		t.Fatalf("mostly compliant Jev evaluation should be accepted: %#v", got)
	}
}

func TestJevCatalogPolicyDoesNotEscalateQualifiedScoresForUncalibratedConfidence(t *testing.T) {
	catalog, err := LoadJevCatalog()
	if err != nil {
		t.Fatal(err)
	}
	dimensions := map[string]JevDimension{
		"facts_support":           {Score: 0.75, RawScore: 3, Confidence: 0.20},
		"contradiction_free":      {Score: 0.75, RawScore: 3, Confidence: 0.25},
		"coverage":                {Score: 0.75, RawScore: 3, Confidence: 0.30},
		"inference_control":       {Score: 0.75, RawScore: 3, Confidence: 0.35},
		"entity_numeric_accuracy": {Score: 0.75, RawScore: 3, Confidence: 0.40},
	}

	got := EvaluateJevGate(dimensions, []string{"facts_support", "contradiction_free"}, catalog.GatePolicy)
	if got.Decision != JevDecisionAccepted {
		t.Fatalf("qualified summary scores should not be rejected by uncalibrated confidence: %#v", got)
	}
	if got.MinimumConfidence != 0.20 {
		t.Fatalf("minimum confidence should remain observable: %#v", got)
	}
}

func TestJevCatalogPolicyEscalatesMaterialDimensionProblem(t *testing.T) {
	catalog, err := LoadJevCatalog()
	if err != nil {
		t.Fatal(err)
	}
	dimensions := map[string]JevDimension{
		"source_support":     {Score: 0.50, RawScore: 2, Confidence: 0.90},
		"contradiction_free": {Score: 1.00, RawScore: 4, Confidence: 0.90},
		"coverage":           {Score: 1.00, RawScore: 4, Confidence: 0.90},
		"inference_control":  {Score: 1.00, RawScore: 4, Confidence: 0.90},
		"specificity":        {Score: 1.00, RawScore: 4, Confidence: 0.90},
	}

	got := EvaluateJevGate(dimensions, []string{"source_support", "contradiction_free"}, catalog.GatePolicy)
	if got.Decision != JevDecisionEscalated || got.EscalationReason != JevEscalationCriticalDimensionLow {
		t.Fatalf("material Jev problem should be escalated: %#v", got)
	}
}
