package repository

import "testing"

func strPtr(v string) *string { return &v }

func TestShouldBackfillOpenRouterUsageLogTrueForNegativeCost(t *testing.T) {
	row := LLMUsageLog{
		Provider:         "openrouter",
		EstimatedCostUSD: -1,
	}

	if !shouldBackfillOpenRouterUsageLog(row) {
		t.Fatal("negative openrouter cost should be selected for backfill")
	}
}

func TestShouldBackfillOpenRouterUsageLogTrueForOpenRouterAutoWithoutBilledCost(t *testing.T) {
	row := LLMUsageLog{
		Provider:       "openrouter",
		RequestedModel: strPtr("openrouter::auto"),
		ResolvedModel:  strPtr("anthropic/claude-4.6-opus-20260205"),
	}

	if !shouldBackfillOpenRouterUsageLog(row) {
		t.Fatal("openrouter::auto resolved rows without billed cost should be selected")
	}
}

func TestShouldBackfillOpenRouterUsageLogSkipsRowsWithBilledCost(t *testing.T) {
	cost := 0.1234
	row := LLMUsageLog{
		Provider:          "openrouter",
		EstimatedCostUSD:  -1,
		OpenRouterCostUSD: &cost,
	}

	if shouldBackfillOpenRouterUsageLog(row) {
		t.Fatal("rows with billed cost should not be selected")
	}
}
