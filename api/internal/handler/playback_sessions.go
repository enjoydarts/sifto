package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type PlaybackSessionsHandler struct {
	service *service.PlaybackSessionsService
}

func NewPlaybackSessionsHandler(service *service.PlaybackSessionsService) *PlaybackSessionsHandler {
	return &PlaybackSessionsHandler{service: service}
}

func (h *PlaybackSessionsHandler) Latest(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, "playback sessions unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	payload, err := h.service.LatestSessions(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func (h *PlaybackSessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, "playback sessions unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	rows, err := h.service.ListHistory(r.Context(), userID, mode, status, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": rows})
}

func (h *PlaybackSessionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input service.StartPlaybackSessionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	input.UserID = middleware.GetUserID(r)
	session, err := h.service.StartSession(r.Context(), input)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, session)
}

func (h *PlaybackSessionsHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.updateWith(r, w, false, false)
}

func (h *PlaybackSessionsHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.updateWith(r, w, true, false)
}

func (h *PlaybackSessionsHandler) Interrupt(w http.ResponseWriter, r *http.Request) {
	h.updateWith(r, w, false, true)
}

func (h *PlaybackSessionsHandler) updateWith(r *http.Request, w http.ResponseWriter, complete bool, interrupt bool) {
	if h == nil || h.service == nil {
		http.Error(w, "playback sessions unavailable", http.StatusInternalServerError)
		return
	}
	var input service.UpdatePlaybackSessionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	input.UserID = middleware.GetUserID(r)
	input.SessionID = strings.TrimSpace(chi.URLParam(r, "id"))
	if input.SessionID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var (
		session any
		err     error
	)
	switch {
	case complete:
		session, err = h.service.CompleteSession(r.Context(), input)
	case interrupt:
		session, err = h.service.InterruptSession(r.Context(), input)
	default:
		session, err = h.service.UpdateProgress(r.Context(), input)
	}
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, session)
}
