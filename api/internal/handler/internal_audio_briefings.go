package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type InternalAudioBriefingsHandler struct {
	repo         *repository.AudioBriefingRepo
	notifier     *service.AudioBriefingPublishedNotifier
	publications *service.PodcastPublicationService
}

func NewInternalAudioBriefingsHandler(repo *repository.AudioBriefingRepo, notifier *service.AudioBriefingPublishedNotifier, publications *service.PodcastPublicationService) *InternalAudioBriefingsHandler {
	return &InternalAudioBriefingsHandler{repo: repo, notifier: notifier, publications: publications}
}

type concatCompleteRequest struct {
	RequestID         string  `json:"request_id"`
	ProviderJobID     *string `json:"provider_job_id"`
	Status            string  `json:"status"`
	AudioObjectKey    *string `json:"audio_object_key"`
	ManifestObjectKey *string `json:"manifest_object_key"`
	BGMObjectKey      *string `json:"bgm_object_key"`
	AudioDurationSec  *int    `json:"audio_duration_sec"`
	ErrorCode         *string `json:"error_code"`
	ErrorMessage      *string `json:"error_message"`
}

func (h *InternalAudioBriefingsHandler) ConcatComplete(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}

	rawToken := extractBearerToken(r)
	if rawToken == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var body concatCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.RequestID = strings.TrimSpace(body.RequestID)
	body.Status = strings.TrimSpace(body.Status)
	if body.RequestID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Status == "" {
		body.Status = "published"
	}
	if body.Status != "published" && body.Status != "failed" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	providerJobID := trimOptionalString(body.ProviderJobID)
	audioObjectKey := trimOptionalString(body.AudioObjectKey)
	manifestObjectKey := trimOptionalString(body.ManifestObjectKey)
	bgmObjectKey := trimOptionalString(body.BGMObjectKey)
	errorCode := trimOptionalString(body.ErrorCode)
	errorMessage := trimOptionalString(body.ErrorMessage)

	if body.Status == "published" && audioObjectKey == nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Status == "failed" && errorCode == nil {
		defaultCode := "concat_failed"
		errorCode = &defaultCode
	}
	if body.AudioDurationSec != nil && *body.AudioDurationSec < 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	job, err := h.repo.FinalizeConcatJob(
		r.Context(),
		jobID,
		body.RequestID,
		service.HashAudioBriefingCallbackToken(rawToken),
		providerJobID,
		body.Status,
		audioObjectKey,
		manifestObjectKey,
		bgmObjectKey,
		body.AudioDurationSec,
		errorCode,
		errorMessage,
	)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUnauthorized):
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			writeRepoError(w, err)
		}
		return
	}
	if job != nil && job.Status == "published" && h.publications != nil {
		updatedJob, publicationErr := h.publications.EnsurePublicCopy(r.Context(), job)
		if publicationErr != nil {
			log.Printf("audio briefing podcast public copy failed job_id=%s user_id=%s err=%v", job.ID, job.UserID, publicationErr)
		} else if updatedJob != nil {
			job = updatedJob
		}
	}
	if job != nil && job.Status == "published" && h.notifier != nil {
		if notifyErr := h.notifier.NotifyPublished(r.Context(), job); notifyErr != nil {
			log.Printf("audio briefing published notification failed job_id=%s user_id=%s err=%v", job.ID, job.UserID, notifyErr)
		}
	}

	writeJSON(w, map[string]any{
		"status": "ok",
		"job":    job,
	})
}

func (h *InternalAudioBriefingsHandler) ChunkHeartbeat(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}

	rawToken := extractBearerToken(r)
	if rawToken == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	chunkID := strings.TrimSpace(chi.URLParam(r, "chunkID"))
	if chunkID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.repo.TouchChunkHeartbeat(r.Context(), chunkID, service.HashAudioBriefingCallbackToken(rawToken)); err != nil {
		switch {
		case errors.Is(err, repository.ErrUnauthorized):
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			writeRepoError(w, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func trimOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func extractBearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
