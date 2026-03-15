package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
)

type fakeReadingGoalStore struct {
	goals []model.ReadingGoal
}

func (f *fakeReadingGoalStore) ListByUser(_ context.Context, userID string) ([]model.ReadingGoal, error) {
	out := make([]model.ReadingGoal, 0, len(f.goals))
	for _, goal := range f.goals {
		if goal.UserID == userID {
			out = append(out, goal)
		}
	}
	return out, nil
}

func (f *fakeReadingGoalStore) Upsert(_ context.Context, goal model.ReadingGoal) (model.ReadingGoal, error) {
	if goal.ID == "" {
		goal.ID = "new-goal"
	}
	if goal.UserID == "" {
		goal.UserID = "u1"
	}
	for i := range f.goals {
		if f.goals[i].ID == goal.ID && f.goals[i].UserID == goal.UserID {
			f.goals[i] = goal
			return goal, nil
		}
	}
	f.goals = append(f.goals, goal)
	return goal, nil
}

func (f *fakeReadingGoalStore) Delete(_ context.Context, userID, id string) error {
	next := f.goals[:0]
	for _, goal := range f.goals {
		if !(goal.UserID == userID && goal.ID == id) {
			next = append(next, goal)
		}
	}
	f.goals = next
	return nil
}

func (f *fakeReadingGoalStore) SetStatus(_ context.Context, userID, id, status string) (model.ReadingGoal, error) {
	for i := range f.goals {
		if f.goals[i].UserID == userID && f.goals[i].ID == id {
			f.goals[i].Status = status
			return f.goals[i], nil
		}
	}
	return model.ReadingGoal{}, nil
}

func TestReadingGoalsHandlerList(t *testing.T) {
	store := &fakeReadingGoalStore{
		goals: []model.ReadingGoal{
			{ID: "g1", UserID: "u1", Title: "AI", Status: "active", Priority: 5},
			{ID: "g2", UserID: "u1", Title: "DB", Status: "archived", Priority: 3},
			{ID: "g3", UserID: "u2", Title: "Other", Status: "active", Priority: 4},
		},
	}
	h := NewReadingGoalsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/settings/reading-goals", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string][]model.ReadingGoal
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp["active"]) != 1 || len(resp["archived"]) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestReadingGoalsHandlerCreateRejectsEighthActiveGoal(t *testing.T) {
	goals := make([]model.ReadingGoal, 0, 7)
	for i := range 7 {
		goals = append(goals, model.ReadingGoal{
			ID:       string(rune('a' + i)),
			UserID:   "u1",
			Title:    "goal",
			Status:   "active",
			Priority: 3,
		})
	}
	store := &fakeReadingGoalStore{goals: goals}
	h := NewReadingGoalsHandler(store)
	body := bytes.NewBufferString(`{"title":"AI","description":"track agents","priority":5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/reading-goals", body)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestReadingGoalsHandlerCreateStoresNormalizedGoal(t *testing.T) {
	store := &fakeReadingGoalStore{}
	h := NewReadingGoalsHandler(store)
	body := bytes.NewBufferString(`{"title":"  AI  ","description":"  follow models  ","priority":4,"due_date":"2026-03-20"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/reading-goals", body)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rr.Code, rr.Body.String())
	}
	if len(store.goals) != 1 {
		t.Fatalf("stored goals = %d, want 1", len(store.goals))
	}
	if store.goals[0].Title != "AI" {
		t.Fatalf("Title = %q, want %q", store.goals[0].Title, "AI")
	}
	if store.goals[0].DueDate == nil || store.goals[0].DueDate.Format("2006-01-02") != "2026-03-20" {
		t.Fatalf("DueDate = %v, want 2026-03-20", store.goals[0].DueDate)
	}
}
