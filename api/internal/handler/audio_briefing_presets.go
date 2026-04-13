package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type audioBriefingPresetsService interface {
	ListAudioBriefingPresets(ctx context.Context, userID string) ([]model.AudioBriefingPreset, error)
	CreateAudioBriefingPreset(ctx context.Context, userID string, in service.SaveAudioBriefingPresetInput) (*model.AudioBriefingPreset, error)
	UpdateAudioBriefingPreset(ctx context.Context, userID, presetID string, in service.SaveAudioBriefingPresetInput) (*model.AudioBriefingPreset, error)
	DeleteAudioBriefingPreset(ctx context.Context, userID, presetID string) error
}

type AudioBriefingPresetsHandler struct {
	settings audioBriefingPresetsService
}

func NewAudioBriefingPresetsHandler(settings audioBriefingPresetsService) *AudioBriefingPresetsHandler {
	return &AudioBriefingPresetsHandler{settings: settings}
}

func (h *AudioBriefingPresetsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.settings == nil {
		http.Error(w, "audio briefing presets unavailable", http.StatusInternalServerError)
		return
	}
	items, err := h.settings.ListAudioBriefingPresets(r.Context(), middleware.GetUserID(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	out := make([]service.AudioBriefingPresetView, 0, len(items))
	for _, item := range items {
		out = append(out, service.AudioBriefingPresetPayload(item))
	}
	writeJSON(w, map[string]any{"presets": out})
}

func (h *AudioBriefingPresetsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.settings == nil {
		http.Error(w, "audio briefing presets unavailable", http.StatusInternalServerError)
		return
	}
	var body service.SaveAudioBriefingPresetInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	preset, err := h.settings.CreateAudioBriefingPreset(r.Context(), middleware.GetUserID(r), body)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	writeJSON(w, map[string]any{"preset": service.AudioBriefingPresetPayload(*preset)})
}

func (h *AudioBriefingPresetsHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.settings == nil {
		http.Error(w, "audio briefing presets unavailable", http.StatusInternalServerError)
		return
	}
	presetID := strings.TrimSpace(chi.URLParam(r, "id"))
	if presetID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var body service.SaveAudioBriefingPresetInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	preset, err := h.settings.UpdateAudioBriefingPreset(r.Context(), middleware.GetUserID(r), presetID, body)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	writeJSON(w, map[string]any{"preset": service.AudioBriefingPresetPayload(*preset)})
}

func (h *AudioBriefingPresetsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.settings == nil {
		http.Error(w, "audio briefing presets unavailable", http.StatusInternalServerError)
		return
	}
	presetID := strings.TrimSpace(chi.URLParam(r, "id"))
	if presetID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := h.settings.DeleteAudioBriefingPreset(r.Context(), middleware.GetUserID(r), presetID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	writeJSON(w, map[string]any{"deleted": true})
}
