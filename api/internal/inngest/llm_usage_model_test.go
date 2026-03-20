package inngest

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestLLMUsageStoredModelPrefersResolvedModelForOpenRouter(t *testing.T) {
	usage := &service.LLMUsage{
		Provider:       "openrouter",
		Model:          "openrouter::auto",
		RequestedModel: "openrouter::auto",
		ResolvedModel:  "openai/gpt-oss-120b",
	}

	if got, want := llmUsageStoredModel(usage), "openai/gpt-oss-120b"; got != want {
		t.Fatalf("llmUsageStoredModel() = %q, want %q", got, want)
	}
}

func TestLLMUsageStoredModelKeepsRequestedModelForNonOpenRouter(t *testing.T) {
	usage := &service.LLMUsage{
		Provider:      "openai",
		Model:         "gpt-5-mini",
		ResolvedModel: "gpt-5.4-mini-2026-03-01",
	}

	if got, want := llmUsageStoredModel(usage), "gpt-5-mini"; got != want {
		t.Fatalf("llmUsageStoredModel() = %q, want %q", got, want)
	}
}

func TestLLMUsageIdempotencyKeyUsesResolvedModelForOpenRouter(t *testing.T) {
	usage := &service.LLMUsage{
		Provider:                 "openrouter",
		Model:                    "openrouter::auto",
		RequestedModel:           "openrouter::auto",
		ResolvedModel:            "openai/gpt-oss-120b",
		PricingModelFamily:       "openrouter::openai/gpt-oss-120b",
		PricingSource:            "openrouter_snapshot",
		InputTokens:              10,
		OutputTokens:             20,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     0,
		EstimatedCostUSD:         0.1,
	}

	keyAuto := llmUsageIdempotencyKey("summary", usage, nil, nil, nil, nil)
	usage.ResolvedModel = "google/gemini-2.5-flash"
	keyGemini := llmUsageIdempotencyKey("summary", usage, nil, nil, nil, nil)

	if keyAuto == keyGemini {
		t.Fatal("idempotency key should change when resolved model changes")
	}
}
