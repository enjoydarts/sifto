package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestMergeBriefingNavigatorCandidatesPrefersEarlierWindowsAndDedupes(t *testing.T) {
	got := mergeBriefingNavigatorCandidates(4,
		[]model.BriefingNavigatorCandidate{
			{ItemID: "fresh-1"},
			{ItemID: "fresh-2"},
		},
		[]model.BriefingNavigatorCandidate{
			{ItemID: "fresh-2"},
			{ItemID: "halfday-1"},
		},
		[]model.BriefingNavigatorCandidate{
			{ItemID: "day-1"},
			{ItemID: "day-2"},
		},
	)

	if len(got) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(got))
	}
	wantOrder := []string{"fresh-1", "fresh-2", "halfday-1", "day-1"}
	for i, want := range wantOrder {
		if got[i].ItemID != want {
			t.Fatalf("candidate %d: want %s, got %s", i, want, got[i].ItemID)
		}
	}
}

func TestBriefingNavigatorCandidatesWindowQueryUsesRandomOrderAndWindowBounds(t *testing.T) {
	query, args := briefingNavigatorCandidatesWindowQuery("user-1", 24, briefingNavigatorCandidateWindow{
		minAge: 1 * time.Hour,
		maxAge: 12 * time.Hour,
	})

	if !strings.Contains(query, "ORDER BY RANDOM()") {
		t.Fatalf("query must randomize candidate order: %s", query)
	}
	if !strings.Contains(query, "make_interval(secs => $2::int)") {
		t.Fatalf("query must filter by max age: %s", query)
	}
	if !strings.Contains(query, "make_interval(secs => $3::int)") {
		t.Fatalf("query must filter by min age fallback boundary: %s", query)
	}
	if got, want := len(args), 4; got != want {
		t.Fatalf("args len = %d, want %d", got, want)
	}
	if got, want := args[1], int((12 * time.Hour).Seconds()); got != want {
		t.Fatalf("max age arg = %v, want %d", got, want)
	}
	if got, want := args[2], int((1 * time.Hour).Seconds()); got != want {
		t.Fatalf("min age arg = %v, want %d", got, want)
	}
	if got, want := args[3], 24; got != want {
		t.Fatalf("limit arg = %v, want %d", got, want)
	}
}
