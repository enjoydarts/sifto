package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type AINavigatorBriefHandler struct {
	service *service.AINavigatorBriefService
}

func NewAINavigatorBriefHandler(service *service.AINavigatorBriefService) *AINavigatorBriefHandler {
	return &AINavigatorBriefHandler{service: service}
}

func (h *AINavigatorBriefHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	slot := strings.TrimSpace(r.URL.Query().Get("slot"))
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	items, err := h.service.ListBriefsByUser(r.Context(), userID, slot, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, model.AINavigatorBriefListResponse{Items: items})
}

func (h *AINavigatorBriefHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	brief, err := h.service.GetBriefDetail(r.Context(), userID, chi.URLParam(r, "id"))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, model.AINavigatorBriefDetailResponse{Brief: brief})
}

func (h *AINavigatorBriefHandler) AppendToSummaryAudioQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	brief, err := h.service.GetBriefDetail(r.Context(), userID, chi.URLParam(r, "id"))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	items := make([]model.Item, 0, len(brief.Items))
	timestamp := brief.CreatedAt
	if brief.GeneratedAt != nil {
		timestamp = *brief.GeneratedAt
	}
	for _, briefItem := range brief.Items {
		items = append(items, model.Item{
			ID:              briefItem.ItemID,
			SourceID:        "",
			URL:             "",
			Title:           nullableString(briefItem.TitleSnapshot),
			TranslatedTitle: nullableString(briefItem.TranslatedTitleSnapshot),
			SourceTitle:     nullableString(briefItem.SourceTitleSnapshot),
			ContentText:     nil,
			Status:          "summarized",
			IsRead:          false,
			IsFavorite:      false,
			FeedbackRating:  0,
			PublishedAt:     nil,
			FetchedAt:       nil,
			CreatedAt:       timestamp,
			UpdatedAt:       timestamp,
		})
	}
	writeJSON(w, map[string]any{
		"brief_id": brief.ID,
		"items":    items,
		"count":    len(items),
	})
}

func nullableString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}
