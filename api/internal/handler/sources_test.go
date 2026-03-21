package handler

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
