package service

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestNormalizeReadingGoalInputTrimsAndDefaults(t *testing.T) {
	input := ReadingGoalInput{
		Title:       "  AI agents  ",
		Description: "  follow core model updates  ",
		Priority:    0,
		DueDate:     " 2026-03-31 ",
	}

	got, err := NormalizeReadingGoalInput(input)
	if err != nil {
		t.Fatalf("NormalizeReadingGoalInput returned error: %v", err)
	}
	if got.Title != "AI agents" {
		t.Fatalf("Title = %q, want %q", got.Title, "AI agents")
	}
	if got.Description != "follow core model updates" {
		t.Fatalf("Description = %q, want %q", got.Description, "follow core model updates")
	}
	if got.Priority != 3 {
		t.Fatalf("Priority = %d, want 3", got.Priority)
	}
	if got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-03-31" {
		t.Fatalf("DueDate = %v, want 2026-03-31", got.DueDate)
	}
}

func TestNormalizeReadingGoalInputRejectsEmptyTitle(t *testing.T) {
	_, err := NormalizeReadingGoalInput(ReadingGoalInput{Title: "   "})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCanActivateAnotherReadingGoalRespectsLimit(t *testing.T) {
	active := make([]model.ReadingGoal, 0, 7)
	for i := range 7 {
		active = append(active, model.ReadingGoal{
			ID:       "g" + string(rune('0'+i)),
			Status:   "active",
			Priority: 3,
		})
	}

	if err := CanActivateAnotherReadingGoal(active, nil); err == nil {
		t.Fatal("expected limit error when creating the 8th active goal")
	}
	if err := CanActivateAnotherReadingGoal(active, &active[0]); err != nil {
		t.Fatalf("expected updating an existing active goal to be allowed, got %v", err)
	}
}

func TestRankTodayQueueItemsOrdersByScoreAndDiversity(t *testing.T) {
	now := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	goals := []model.ReadingGoal{
		{ID: "goal-ai", Title: "AI", Description: "agent tooling", Status: "active", Priority: 5},
	}
	candidates := []model.TodayQueueCandidate{
		{
			Item: model.Item{
				ID:                   "item-1",
				Title:                strPtr("AI agents weekly"),
				SummaryTopics:        []string{"AI", "Agents"},
				PersonalScore:        floatPtr(0.61),
				RecommendationReason: strPtr("Matches your AI workflow"),
				CreatedAt:            now.Add(-2 * time.Hour),
			},
			LastSkippedAt: nil,
		},
		{
			Item: model.Item{
				ID:            "item-2",
				Title:         strPtr("Databases at scale"),
				SummaryTopics: []string{"Databases"},
				PersonalScore: floatPtr(0.59),
				CreatedAt:     now.Add(-1 * time.Hour),
			},
		},
		{
			Item: model.Item{
				ID:            "item-3",
				Title:         strPtr("Another AI story"),
				SummaryTopics: []string{"AI"},
				PersonalScore: floatPtr(0.60),
				CreatedAt:     now.Add(-30 * time.Minute),
			},
			LastSkippedAt: timePtr(now.Add(-20 * time.Minute)),
		},
	}

	got := RankTodayQueueItems(candidates, goals, 2, now)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Item.ID != "item-1" {
		t.Fatalf("first item = %s, want item-1", got[0].Item.ID)
	}
	if got[1].Item.ID != "item-2" {
		t.Fatalf("second item = %s, want item-2", got[1].Item.ID)
	}
	if len(got[0].ReasonLabels) == 0 {
		t.Fatal("expected reason labels for ranked item")
	}
	if got[0].EstimatedReadingMinutes <= 0 {
		t.Fatalf("EstimatedReadingMinutes = %d, want > 0", got[0].EstimatedReadingMinutes)
	}
}

func strPtr(v string) *string        { return &v }
func floatPtr(v float64) *float64    { return &v }
func timePtr(v time.Time) *time.Time { return &v }
