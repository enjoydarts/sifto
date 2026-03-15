package service

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestWeeklyReviewBuildsReadNoteInsightCounts(t *testing.T) {
	snapshot := BuildWeeklyReviewSnapshot("u1", "2026-03-09", "2026-03-15", WeeklyReviewInputs{
		ReadCount:     12,
		NoteCount:     3,
		InsightCount:  2,
		FavoriteCount: 4,
	})

	if snapshot.ReadCount != 12 || snapshot.NoteCount != 3 || snapshot.InsightCount != 2 || snapshot.FavoriteCount != 4 {
		t.Fatalf("snapshot counts = %+v", snapshot)
	}
}

func TestWeeklyReviewFindsDominantTopics(t *testing.T) {
	snapshot := BuildWeeklyReviewSnapshot("u1", "2026-03-09", "2026-03-15", WeeklyReviewInputs{
		Topics: []model.WeeklyReviewTopic{
			{Topic: "AI", Count: 5},
			{Topic: "Agents", Count: 3},
			{Topic: "Infra", Count: 1},
		},
	})

	if len(snapshot.DominantTopics) != 3 {
		t.Fatalf("dominant topics len = %d, want 3", len(snapshot.DominantTopics))
	}
	if snapshot.DominantTopics[0].Topic != "AI" {
		t.Fatalf("top topic = %s, want AI", snapshot.DominantTopics[0].Topic)
	}
}

func TestWeeklyReviewIncludesMissedHighValueItems(t *testing.T) {
	snapshot := BuildWeeklyReviewSnapshot("u1", "2026-03-09", "2026-03-15", WeeklyReviewInputs{
		MissedHighValue: []model.Item{
			{ID: "item-1", URL: "https://example.com/1"},
			{ID: "item-2", URL: "https://example.com/2"},
		},
	})

	if len(snapshot.MissedHighValue) != 2 {
		t.Fatalf("missed len = %d, want 2", len(snapshot.MissedHighValue))
	}
}
