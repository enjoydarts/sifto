package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type SummaryAudioPlayerHandler struct {
	service *service.SummaryAudioPlayerService
}

func NewSummaryAudioPlayerHandler(service *service.SummaryAudioPlayerService) *SummaryAudioPlayerHandler {
	return &SummaryAudioPlayerHandler{service: service}
}

func (h *SummaryAudioPlayerHandler) Synthesize(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, "summary audio unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	itemID := strings.TrimSpace(chi.URLParam(r, "id"))
	if itemID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	ctx := service.SummaryAudioRequestContext(r.Context())
	if ctx == nil {
		ctx = context.Background()
	}
	resp, err := h.service.Synthesize(ctx, userID, itemID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, service.ErrGeminiTTSNotAllowed):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, service.ErrSummaryAudioMissingSummary), errors.Is(err, service.ErrSummaryAudioMissingVoice):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, resp)
}
