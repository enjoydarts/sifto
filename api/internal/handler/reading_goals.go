package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type readingGoalStore interface {
	ListByUser(ctx context.Context, userID string) ([]model.ReadingGoal, error)
	Upsert(ctx context.Context, goal model.ReadingGoal) (model.ReadingGoal, error)
	Delete(ctx context.Context, userID, id string) error
	SetStatus(ctx context.Context, userID, id, status string) (model.ReadingGoal, error)
}

type ReadingGoalsHandler struct {
	store readingGoalStore
}

func NewReadingGoalsHandler(store readingGoalStore) *ReadingGoalsHandler {
	return &ReadingGoalsHandler{store: store}
}

func (h *ReadingGoalsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	goals, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	active := make([]model.ReadingGoal, 0, len(goals))
	archived := make([]model.ReadingGoal, 0, len(goals))
	for _, goal := range goals {
		if goal.Status == "archived" {
			archived = append(archived, goal)
		} else {
			active = append(active, goal)
		}
	}
	writeJSON(w, map[string]any{
		"active":   readingGoalPayloads(active),
		"archived": readingGoalPayloads(archived),
	})
}

func (h *ReadingGoalsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		DueDate     string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	goal, err := service.NormalizeReadingGoalInput(service.ReadingGoalInput(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	goals, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := service.CanActivateAnotherReadingGoal(goals, nil); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	goal.UserID = userID
	stored, err := h.store.Upsert(r.Context(), goal)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, readingGoalPayload(stored))
}

func (h *ReadingGoalsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		DueDate     string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	goal, err := service.NormalizeReadingGoalInput(service.ReadingGoalInput(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	goals, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var current *model.ReadingGoal
	for i := range goals {
		if goals[i].ID == id {
			current = &goals[i]
			break
		}
	}
	if current == nil {
		writeRepoError(w, repository.ErrNotFound)
		return
	}
	if err := service.CanActivateAnotherReadingGoal(goals, current); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	goal.ID = id
	goal.UserID = userID
	goal.Status = current.Status
	stored, err := h.store.Upsert(r.Context(), goal)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, readingGoalPayload(stored))
}

func (h *ReadingGoalsHandler) Archive(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, "archived")
}

func (h *ReadingGoalsHandler) Restore(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	goals, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := service.CanActivateAnotherReadingGoal(goals, nil); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.setStatus(w, r, "active")
}

func (h *ReadingGoalsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if err := h.store.Delete(r.Context(), userID, chi.URLParam(r, "id")); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ReadingGoalsHandler) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	userID := middleware.GetUserID(r)
	goal, err := h.store.SetStatus(r.Context(), userID, chi.URLParam(r, "id"), status)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, readingGoalPayload(goal))
}

func readingGoalPayloads(goals []model.ReadingGoal) []map[string]any {
	out := make([]map[string]any, 0, len(goals))
	for _, goal := range goals {
		out = append(out, readingGoalPayload(goal))
	}
	return out
}

func readingGoalPayload(goal model.ReadingGoal) map[string]any {
	var dueDate *string
	if goal.DueDate != nil {
		formatted := goal.DueDate.In(time.UTC).Format("2006-01-02")
		dueDate = &formatted
	}
	return map[string]any{
		"id":          goal.ID,
		"user_id":     goal.UserID,
		"title":       goal.Title,
		"description": goal.Description,
		"priority":    goal.Priority,
		"status":      goal.Status,
		"due_date":    dueDate,
		"created_at":  goal.CreatedAt,
		"updated_at":  goal.UpdatedAt,
	}
}
