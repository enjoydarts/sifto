package repository

import (
	"strings"
	"testing"
	"time"
)

func TestAudioBriefingCandidateItemsQueryPrioritizesRecentFetchedWindow(t *testing.T) {
	windowStart := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
	query, args := audioBriefingCandidateItemsQuery("user-1", windowStart, 8)

	if !strings.Contains(query, "COALESCE(i.fetched_at, i.created_at) >= $2") {
		t.Fatalf("query must filter items from the fetched-time slot boundary: %s", query)
	}
	if !strings.Contains(query, "sm.score DESC NULLS LAST") {
		t.Fatalf("query must prioritize higher scored summaries within the slot: %s", query)
	}
	if !strings.Contains(query, "COALESCE(i.fetched_at, i.created_at) DESC") {
		t.Fatalf("query must keep fetched recency as a secondary tiebreaker: %s", query)
	}
	if !strings.Contains(query, "COALESCE(i.published_at, i.created_at) DESC") {
		t.Fatalf("query must retain published recency ordering after score and fetched recency: %s", query)
	}
	if got, want := len(args), 3; got != want {
		t.Fatalf("args len = %d, want %d", got, want)
	}
	if got, ok := args[1].(time.Time); !ok || !got.Equal(windowStart) {
		t.Fatalf("windowStart arg = %v, want %v", args[1], windowStart)
	}
	if got, want := args[2], 8; got != want {
		t.Fatalf("limit arg = %v, want %d", got, want)
	}
}

func TestAudioBriefingCandidateItemsQueryDefaultsLookbackHours(t *testing.T) {
	_, args := audioBriefingCandidateItemsQuery("user-1", time.Time{}, 6)
	if got, ok := args[1].(time.Time); !ok || got.IsZero() {
		t.Fatalf("default windowStart arg = %v, want non-zero time", args[1])
	}
}
