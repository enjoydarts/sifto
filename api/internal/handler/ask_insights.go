package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type AskInsightsHandler struct {
	repo *repository.AskInsightRepo
}

func NewAskInsightsHandler(repo *repository.AskInsightRepo) *AskInsightsHandler {
	return &AskInsightsHandler{repo: repo}
}

func (h *AskInsightsHandler) Save(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		Title   string   `json:"title"`
		Body    string   `json:"body"`
		Query   string   `json:"query"`
		GoalID  *string  `json:"goal_id"`
		Tags    []string `json:"tags"`
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Title) == "" || strings.TrimSpace(body.Body) == "" {
		http.Error(w, "title and body are required", http.StatusBadRequest)
		return
	}
	insight, err := h.repo.Save(r.Context(), model.AskInsight{
		UserID: userID,
		Title:  strings.TrimSpace(body.Title),
		Body:   strings.TrimSpace(body.Body),
		Query:  strings.TrimSpace(body.Query),
		GoalID: body.GoalID,
		Tags:   normalizeStringSlice(body.Tags),
	}, body.ItemIDs)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, insight)
}

func (h *AskInsightsHandler) ListRecent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 6)
	insights, err := h.repo.ListRecent(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"insights": insights})
}

func (h *AskInsightsHandler) Delete(w http.ResponseWriter, r *http.Request, insightID string) {
	userID := middleware.GetUserID(r)
	if err := h.repo.Delete(r.Context(), userID, insightID); err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "id": insightID})
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
