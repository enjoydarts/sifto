package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type AudioBriefingsHandler struct {
	repo           *repository.AudioBriefingRepo
	orchestrator   *service.AudioBriefingOrchestrator
	voiceRunner    *service.AudioBriefingVoiceRunner
	concatStarter  *service.AudioBriefingConcatStarter
	deleteService  *service.AudioBriefingDeleteService
	eventPublisher *service.EventPublisher
	worker         *service.WorkerClient
}

type audioBriefingUsedTTSPayload struct {
	Provider          string  `json:"provider"`
	TTSModel          *string `json:"tts_model,omitempty"`
	HostVoiceModel    *string `json:"host_voice_model,omitempty"`
	PartnerVoiceModel *string `json:"partner_voice_model,omitempty"`
}

func NewAudioBriefingsHandler(
	repo *repository.AudioBriefingRepo,
	orchestrator *service.AudioBriefingOrchestrator,
	voiceRunner *service.AudioBriefingVoiceRunner,
	concatStarter *service.AudioBriefingConcatStarter,
	deleteService *service.AudioBriefingDeleteService,
	eventPublisher *service.EventPublisher,
	worker *service.WorkerClient,
) *AudioBriefingsHandler {
	return &AudioBriefingsHandler{
		repo:           repo,
		orchestrator:   orchestrator,
		voiceRunner:    voiceRunner,
		concatStarter:  concatStarter,
		deleteService:  deleteService,
		eventPublisher: eventPublisher,
		worker:         worker,
	}
}

func (h *AudioBriefingsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	limit := 24
	tab := normalizeAudioBriefingListTabParam(r.URL.Query().Get("tab"))
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	fetchLimit := limit * 4
	if fetchLimit < 60 {
		fetchLimit = 60
	}
	if fetchLimit > 200 {
		fetchLimit = 200
	}
	rows, err := h.repo.ListJobsByUser(r.Context(), userID, fetchLimit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	filtered := make([]model.AudioBriefingJob, 0, limit)
	for _, row := range rows {
		if !service.AudioBriefingListTabMatches(&row, tab) {
			continue
		}
		filtered = append(filtered, row)
		if len(filtered) >= limit {
			break
		}
	}
	writeJSON(w, map[string]any{"items": filtered})
}

func (h *AudioBriefingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	jobID := chi.URLParam(r, "id")
	payload, err := h.loadDetail(r.Context(), userID, jobID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func (h *AudioBriefingsHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil || h.orchestrator == nil || h.eventPublisher == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	job, err := h.orchestrator.GenerateManual(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.enqueueRun(userID, job.ID, "manual"); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	payload, err := h.loadDetail(r.Context(), userID, job.ID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func (h *AudioBriefingsHandler) StartConcat(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil || h.concatStarter == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := h.concatStarter.Start(r.Context(), userID, jobID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		case errors.Is(err, service.ErrAudioConcatRunnerDisabled):
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	payload, err := h.loadDetail(r.Context(), userID, jobID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func (h *AudioBriefingsHandler) Resume(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil || h.orchestrator == nil || h.eventPublisher == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	job, err := h.orchestrator.Resume(r.Context(), userID, jobID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if err := h.enqueueRun(userID, job.ID, "resume"); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	payload, err := h.loadDetail(r.Context(), userID, job.ID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func (h *AudioBriefingsHandler) Archive(w http.ResponseWriter, r *http.Request) {
	h.updateArchiveStatus(w, r, "archived")
}

func (h *AudioBriefingsHandler) Unarchive(w http.ResponseWriter, r *http.Request) {
	h.updateArchiveStatus(w, r, "active")
}

func (h *AudioBriefingsHandler) enqueueRun(userID, jobID, trigger string) error {
	if h.eventPublisher == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.eventPublisher.SendAudioBriefingRunE(ctx, userID, jobID, trigger)
}

func (h *AudioBriefingsHandler) StartVoicing(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil || h.voiceRunner == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if _, err := h.voiceRunner.Start(r.Context(), userID, jobID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	payload, err := h.loadDetail(r.Context(), userID, jobID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func (h *AudioBriefingsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil || h.deleteService == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := h.deleteService.Delete(r.Context(), userID, jobID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AudioBriefingsHandler) loadDetail(ctx context.Context, userID, jobID string) (map[string]any, error) {
	job, err := h.repo.GetJobByID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	items, err := h.repo.ListJobItems(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	chunks, err := h.repo.ListJobChunks(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	audioURL := resolvePlayableAudioURL(ctx, h.worker, job.R2StorageBucket, job.R2AudioObjectKey)
	return map[string]any{
		"job":               job,
		"items":             items,
		"chunks":            chunks,
		"used_tts":          audioBriefingUsedTTS(chunks),
		"audio_url":         audioURL,
		"delete_allowed":    service.AudioBriefingDeleteAllowed(job),
		"resume_allowed":    service.AudioBriefingResumeAllowed(job),
		"archive_allowed":   service.AudioBriefingArchiveAllowed(job),
		"unarchive_allowed": service.AudioBriefingUnarchiveAllowed(job),
	}, nil
}

func audioBriefingUsedTTS(chunks []model.AudioBriefingScriptChunk) *audioBriefingUsedTTSPayload {
	if len(chunks) == 0 {
		return nil
	}
	payload := &audioBriefingUsedTTSPayload{}
	for _, chunk := range chunks {
		provider := strings.TrimSpace(derefString(chunk.TTSProvider))
		if provider == "" {
			continue
		}
		if payload.Provider == "" {
			payload.Provider = provider
		}
		if payload.TTSModel == nil {
			if model := strings.TrimSpace(derefString(chunk.TTSModel)); model != "" {
				payload.TTSModel = &model
			}
		}
		speaker := strings.TrimSpace(derefString(chunk.Speaker))
		voiceModel := strings.TrimSpace(derefString(chunk.VoiceModel))
		if voiceModel == "" {
			continue
		}
		switch speaker {
		case "partner":
			if payload.PartnerVoiceModel == nil {
				payload.PartnerVoiceModel = &voiceModel
			}
		default:
			if payload.HostVoiceModel == nil {
				payload.HostVoiceModel = &voiceModel
			}
		}
	}
	if payload.Provider == "" {
		return nil
	}
	return payload
}

func (h *AudioBriefingsHandler) updateArchiveStatus(w http.ResponseWriter, r *http.Request, archiveStatus string) {
	if h.repo == nil {
		http.Error(w, "audio briefing unavailable", http.StatusInternalServerError)
		return
	}
	userID := middleware.GetUserID(r)
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	job, err := h.repo.GetJobByID(r.Context(), userID, jobID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	switch archiveStatus {
	case "archived":
		if !service.AudioBriefingArchiveAllowed(job) {
			http.Error(w, repository.ErrInvalidState.Error(), http.StatusConflict)
			return
		}
	case "active":
		if !service.AudioBriefingUnarchiveAllowed(job) {
			http.Error(w, repository.ErrInvalidState.Error(), http.StatusConflict)
			return
		}
	default:
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if _, err := h.repo.UpdateArchiveStatus(r.Context(), userID, jobID, archiveStatus); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, repository.ErrInvalidState), errors.Is(err, repository.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	payload, err := h.loadDetail(r.Context(), userID, jobID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, payload)
}

func normalizeAudioBriefingListTabParam(value string) string {
	switch strings.TrimSpace(value) {
	case "archived":
		return "archived"
	case "pending":
		return "pending"
	case "storage":
		return "storage"
	default:
		return "published"
	}
}

func resolvePlayableAudioURL(ctx context.Context, worker *service.WorkerClient, bucket string, objectKey *string) *string {
	if objectKey == nil {
		return nil
	}
	value := strings.TrimSpace(*objectKey)
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return &value
	}
	if worker == nil || value == "" {
		return nil
	}
	resp, err := worker.PresignAudioBriefingObjectInBucket(ctx, value, service.NormalizeAudioBriefingStorageBucket(bucket), 3600)
	if err != nil || resp == nil {
		return nil
	}
	audioURL := strings.TrimSpace(resp.AudioURL)
	if audioURL == "" {
		return nil
	}
	return &audioURL
}
