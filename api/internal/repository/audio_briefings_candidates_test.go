package repository

import (
	"strings"
	"testing"
)

func TestAudioBriefingCandidateItemsQueryPrioritizesRecentFetchedWindow(t *testing.T) {
	query, args := audioBriefingCandidateItemsQuery("user-1", 3, 8)

	if !strings.Contains(query, "COALESCE(i.fetched_at, i.created_at) >= NOW() - make_interval(hours => $2::int)") {
		t.Fatalf("query must prioritize recently fetched items: %s", query)
	}
	if !strings.Contains(query, "COALESCE(i.fetched_at, i.created_at) DESC") {
		t.Fatalf("query must sort recent fetched items first within the priority bucket: %s", query)
	}
	if !strings.Contains(query, "COALESCE(i.published_at, i.created_at) DESC") {
		t.Fatalf("query must retain published recency ordering after fetched priority: %s", query)
	}
	if got, want := len(args), 3; got != want {
		t.Fatalf("args len = %d, want %d", got, want)
	}
	if got, want := args[1], 3; got != want {
		t.Fatalf("interval arg = %v, want %d", got, want)
	}
	if got, want := args[2], 8; got != want {
		t.Fatalf("limit arg = %v, want %d", got, want)
	}
}

func TestAudioBriefingCandidateItemsQueryDefaultsIntervalHours(t *testing.T) {
	_, args := audioBriefingCandidateItemsQuery("user-1", 0, 6)
	if got, want := args[1], 6; got != want {
		t.Fatalf("default interval arg = %v, want %d", got, want)
	}
}
