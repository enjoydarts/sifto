package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPopulateSourceSuggestionsFromProbesAddsFallbackCandidates(t *testing.T) {
	probes := []probeSeed{
		{
			SourceID: "src-1",
			ProbeURL: "https://example.com/",
			Reason:   "同一サイトのトップページから発見",
		},
	}
	registered := map[string]bool{
		normalizeFeedURL("https://example.com/feed.xml"): true,
	}
	cands := map[string]*sourceSuggestionAgg{}

	populateSourceSuggestionsFromProbes(
		context.Background(),
		probes,
		[]string{"ai"},
		registered,
		cands,
		func() time.Duration { return 2 * time.Second },
		func(_ context.Context, raw string) ([]FeedCandidate, error) {
			if raw != "https://example.com/" {
				t.Fatalf("probe url = %q, want %q", raw, "https://example.com/")
			}
			title := "Example AI Feed"
			return []FeedCandidate{
				{URL: "https://example.com/feed.xml", Title: &title},
				{URL: "https://example.com/ai.xml", Title: &title},
			}, nil
		},
	)

	if len(cands) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(cands))
	}
	got := cands[normalizeFeedURL("https://example.com/ai.xml")]
	if got == nil {
		t.Fatalf("expected fallback candidate to be added")
	}
	if got.Score != 6 {
		t.Fatalf("score = %d, want 6", got.Score)
	}
	if !got.SeedSourceIDs["src-1"] {
		t.Fatalf("expected seed source id to be recorded")
	}
	if !got.MatchedTopics["ai"] {
		t.Fatalf("expected matched topic to be recorded")
	}
}

func TestPopulateSourceSuggestionsFromProbesSkipsOnBudgetExhaustion(t *testing.T) {
	cands := map[string]*sourceSuggestionAgg{}
	called := false

	populateSourceSuggestionsFromProbes(
		context.Background(),
		[]probeSeed{{SourceID: "src-1", ProbeURL: "https://example.com/", Reason: "root"}},
		nil,
		map[string]bool{},
		cands,
		func() time.Duration { return 0 },
		func(_ context.Context, _ string) ([]FeedCandidate, error) {
			called = true
			return nil, errors.New("should not be called")
		},
	)

	if called {
		t.Fatalf("discover should not be called when budget is exhausted")
	}
	if len(cands) != 0 {
		t.Fatalf("candidate count = %d, want 0", len(cands))
	}
}

func TestSelectSourceSuggestionLLMResolvesOpenAICompatibleProviders(t *testing.T) {
	tests := []struct {
		name  string
		model string
		check func(t *testing.T, got resolvedProviderKeys)
	}{
		{
			name:  "moonshot",
			model: "kimi-k2-turbo-preview",
			check: func(t *testing.T, got resolvedProviderKeys) {
				if got.MoonshotAPIKey == nil || *got.MoonshotAPIKey != "moonshot-key" {
					t.Fatalf("MoonshotAPIKey = %v", got.MoonshotAPIKey)
				}
			},
		},
		{
			name:  "openrouter",
			model: "openrouter::openai/gpt-5.4-mini",
			check: func(t *testing.T, got resolvedProviderKeys) {
				if got.OpenRouterAPIKey == nil || *got.OpenRouterAPIKey != "openrouter-key" {
					t.Fatalf("OpenRouterAPIKey = %v", got.OpenRouterAPIKey)
				}
			},
		},
		{
			name:  "poe",
			model: "poe::claude-sonnet-4",
			check: func(t *testing.T, got resolvedProviderKeys) {
				if got.PoeAPIKey == nil || *got.PoeAPIKey != "poe-key" {
					t.Fatalf("PoeAPIKey = %v", got.PoeAPIKey)
				}
			},
		},
		{
			name:  "siliconflow",
			model: "siliconflow::Qwen/Qwen3-Next-80B-A3B-Instruct",
			check: func(t *testing.T, got resolvedProviderKeys) {
				if got.SiliconFlowAPIKey == nil || *got.SiliconFlowAPIKey != "siliconflow-key" {
					t.Fatalf("SiliconFlowAPIKey = %v", got.SiliconFlowAPIKey)
				}
			},
		},
		{
			name:  "deepinfra",
			model: "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo",
			check: func(t *testing.T, got resolvedProviderKeys) {
				if got.DeepInfraAPIKey == nil || *got.DeepInfraAPIKey != "deepinfra-key" {
					t.Fatalf("DeepInfraAPIKey = %v", got.DeepInfraAPIKey)
				}
				if got.OpenAIAPIKey == nil || *got.OpenAIAPIKey != "deepinfra-key" {
					t.Fatalf("OpenAIAPIKey = %v, want deepinfra-key passthrough", got.OpenAIAPIKey)
				}
			},
		},
		{
			name:  "cerebras",
			model: "cerebras::gpt-oss-120b",
			check: func(t *testing.T, got resolvedProviderKeys) {
				if got.CerebrasAPIKey == nil || *got.CerebrasAPIKey != "cerebras-key" {
					t.Fatalf("CerebrasAPIKey = %v", got.CerebrasAPIKey)
				}
				if got.OpenAIAPIKey == nil || *got.OpenAIAPIKey != "cerebras-key" {
					t.Fatalf("OpenAIAPIKey = %v, want cerebras-key passthrough", got.OpenAIAPIKey)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectSourceSuggestionLLM(
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				strPtr("moonshot-key"),
				nil,
				nil,
				nil,
				nil,
				strPtr("openrouter-key"),
				strPtr("poe-key"),
				strPtr("siliconflow-key"),
				nil,
				strPtr("deepinfra-key"),
				strPtr("cerebras-key"),
				strPtr("openai-key"),
				strPtr(tt.model),
			)
			tt.check(t, got)
		})
	}
}

func TestSourceSuggestionLLMStageTimeoutsAreLongEnoughForReasoningModels(t *testing.T) {
	if sourceSuggestionSeedGenerationTimeout != 120*time.Second {
		t.Fatalf("sourceSuggestionSeedGenerationTimeout = %s, want 120s", sourceSuggestionSeedGenerationTimeout)
	}
	if sourceSuggestionRankTimeout != 120*time.Second {
		t.Fatalf("sourceSuggestionRankTimeout = %s, want 120s", sourceSuggestionRankTimeout)
	}
}
