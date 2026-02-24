package service

import "testing"

func TestEstimateOpenAIEmbeddingCostUSD(t *testing.T) {
	got, err := EstimateOpenAIEmbeddingCostUSD("text-embedding-3-small", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Provider != "openai" {
		t.Fatalf("provider = %q", got.Provider)
	}
	if got.PricingSource == "" {
		t.Fatal("pricing source is empty")
	}
	want := 0.00002
	if got.EstimatedCostUSD != want {
		t.Fatalf("estimated cost = %.8f, want %.8f", got.EstimatedCostUSD, want)
	}
}

func TestEstimateOpenAIEmbeddingCostUSDUnsupportedModel(t *testing.T) {
	if _, err := EstimateOpenAIEmbeddingCostUSD("unknown-model", 1000); err == nil {
		t.Fatal("expected error for unsupported model")
	}
}
