package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
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
		case errors.Is(err, service.ErrSummaryAudioMissingSummary), errors.Is(err, service.ErrSummaryAudioMissingVoice), errors.Is(err, service.ErrSummaryAudioMissingModel):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeSummaryAudioBinary(w, resp)
}

func writeSummaryAudioBinary(w http.ResponseWriter, resp *service.SummaryAudioSynthesis) {
	if resp == nil || len(resp.AudioBytes) == 0 {
		http.Error(w, "summary audio response missing audio", http.StatusInternalServerError)
		return
	}
	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(resp.AudioBytes)))
	w.Header().Set("Cache-Control", "no-store")
	if resp.DurationSec > 0 {
		w.Header().Set("X-Summary-Audio-Duration-Sec", strconv.Itoa(resp.DurationSec))
	}
	if text := strings.TrimSpace(derefString(resp.PreprocessedText)); text != "" {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(text))
		if len(encoded) <= 7000 {
			w.Header().Set("X-Summary-Audio-Preprocessed-Text-B64", encoded)
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.AudioBytes)
}
