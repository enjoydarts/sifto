package service

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestReviewQueueSchedulesAt1d7d30d(t *testing.T) {
	base := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	stages := BuildReviewSchedules(base)
	if len(stages) != 3 {
		t.Fatalf("len(stages) = %d, want 3", len(stages))
	}
	if got := stages[0].DueAt.Sub(base); got != 24*time.Hour {
		t.Fatalf("first due = %s, want 24h", got)
	}
	if got := stages[1].DueAt.Sub(base); got != 7*24*time.Hour {
		t.Fatalf("second due = %s, want 7d", got)
	}
	if got := stages[2].DueAt.Sub(base); got != 30*24*time.Hour {
		t.Fatalf("third due = %s, want 30d", got)
	}
}

func TestReviewQueuePrioritizesFavoriteAndNoteItems(t *testing.T) {
	items := []model.ReviewQueueItem{
		{
			ID:           "normal",
			SourceSignal: "read",
			Item:         model.Item{ID: "normal"},
		},
		{
			ID:           "favorite",
			SourceSignal: "favorite",
			Item:         model.Item{ID: "favorite", IsFavorite: true},
		},
		{
			ID:           "note",
			SourceSignal: "note",
			Item:         model.Item{ID: "note"},
			ReasonLabels: []string{"note"},
		},
	}

	ranked := RankReviewQueue(items)
	if ranked[0].ID != "favorite" {
		t.Fatalf("first = %s, want favorite", ranked[0].ID)
	}
	if ranked[1].ID != "note" {
		t.Fatalf("second = %s, want note", ranked[1].ID)
	}
}

func TestReviewQueueSuppressesRecentlySurfacedItems(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	recent := now.Add(-30 * time.Minute)
	old := now.Add(-48 * time.Hour)
	items := []model.ReviewQueueItem{
		{ID: "recent", LastSurfacedAt: &recent},
		{ID: "old", LastSurfacedAt: &old},
	}

	ranked := RankReviewQueueAt(items, now)
	if ranked[0].ID != "old" {
		t.Fatalf("first = %s, want old", ranked[0].ID)
	}
}
