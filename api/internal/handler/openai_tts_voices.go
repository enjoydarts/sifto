package handler

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type openAITTSVoiceCatalogFetcher interface {
	FetchVoices(ctx context.Context) ([]repository.OpenAITTSVoiceSnapshot, error)
}

type OpenAITTSVoicesHandler struct {
	repo               *repository.OpenAITTSVoiceRepo
	providerUpdateRepo *repository.ProviderModelUpdateRepo
	service            openAITTSVoiceCatalogFetcher
}

func NewOpenAITTSVoicesHandler(repo *repository.OpenAITTSVoiceRepo, providerUpdateRepo *repository.ProviderModelUpdateRepo, svc openAITTSVoiceCatalogFetcher) *OpenAITTSVoicesHandler {
	return &OpenAITTSVoicesHandler{repo: repo, providerUpdateRepo: providerUpdateRepo, service: svc}
}

func (h *OpenAITTSVoicesHandler) List(w http.ResponseWriter, r *http.Request) {
	voices, latestRun, err := h.repo.ListLatestSuccessfulSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "openai")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	if voices == nil {
		voices = make([]repository.OpenAITTSVoiceSnapshot, 0)
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"voices":                voices,
		"latest_change_summary": latestChangeSummary,
	})
}

func (h *OpenAITTSVoicesHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if openAITTSVoiceSyncRunIsStale(run, time.Now().UTC()) {
		if err := h.repo.FailSyncRun(r.Context(), run.ID, "OpenAI TTS voice sync stalled"); err != nil {
			writeRepoError(w, err)
			return
		}
		run = nil
	}
	if run != nil && run.Status != "running" {
		run = nil
	}
	writeJSON(w, map[string]any{"run": run})
}

func (h *OpenAITTSVoicesHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if run, err := h.repo.GetLatestRun(r.Context()); err == nil && run != nil {
		if run.Status == "running" {
			if openAITTSVoiceSyncRunIsStale(run, time.Now().UTC()) {
				if err := h.repo.FailSyncRun(r.Context(), run.ID, "OpenAI TTS voice sync interrupted"); err != nil {
					writeRepoError(w, err)
					return
				}
			} else {
				http.Error(w, "openai tts voice sync already running", http.StatusConflict)
				return
			}
		}
	} else if err != nil {
		writeRepoError(w, err)
		return
	}

	previousVoiceIDs, err := h.previousVoiceIDs(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}

	syncRunID, err := h.repo.StartSyncRun(r.Context(), "manual")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	voices, fetchErr := h.service.FetchVoices(r.Context())
	if fetchErr != nil {
		msg := fetchErr.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, 0, 0, &msg)
		if h.providerUpdateRepo != nil {
			_ = h.providerUpdateRepo.UpsertSnapshot(r.Context(), "openai", previousVoiceIDs, "failed", &msg)
		}
		http.Error(w, fetchErr.Error(), http.StatusBadGateway)
		return
	}

	fetchedAt := time.Now().UTC()
	if err := h.repo.InsertSnapshots(r.Context(), syncRunID, fetchedAt, voices); err != nil {
		msg := err.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), 0, &msg)
		h.markProviderSnapshotFailed(r.Context(), previousVoiceIDs, msg)
		writeRepoError(w, err)
		return
	}

	if h.providerUpdateRepo != nil {
		voiceIDs := openAITTSVoiceIDs(voices)
		if err := h.providerUpdateRepo.UpsertSnapshot(r.Context(), "openai", voiceIDs, "ok", nil); err != nil {
			msg := err.Error()
			_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), len(voices), &msg)
			h.markProviderSnapshotFailed(r.Context(), previousVoiceIDs, msg)
			writeRepoError(w, err)
			return
		}
		if err := h.insertOpenAITTSVoiceChangeEvents(r.Context(), "manual", previousVoiceIDs, voices); err != nil {
			msg := err.Error()
			_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), len(voices), &msg)
			h.markProviderSnapshotFailed(r.Context(), previousVoiceIDs, msg)
			writeRepoError(w, err)
			return
		}
	}
	if err := h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), len(voices), nil); err != nil {
		msg := err.Error()
		h.markProviderSnapshotFailed(r.Context(), previousVoiceIDs, msg)
		writeRepoError(w, err)
		return
	}

	h.List(w, r)
}

func (h *OpenAITTSVoicesHandler) previousVoiceIDs(ctx context.Context) ([]string, error) {
	if h.providerUpdateRepo == nil {
		return nil, nil
	}
	snapshot, err := h.providerUpdateRepo.GetSnapshot(ctx, "openai")
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	return append([]string{}, snapshot.Models...), nil
}

func (h *OpenAITTSVoicesHandler) insertOpenAITTSVoiceChangeEvents(ctx context.Context, trigger string, previousVoiceIDs []string, latest []repository.OpenAITTSVoiceSnapshot) error {
	if h.providerUpdateRepo == nil {
		return nil
	}
	prevSet := make(map[string]struct{}, len(previousVoiceIDs))
	for _, voiceID := range previousVoiceIDs {
		voiceID = strings.TrimSpace(voiceID)
		if voiceID != "" {
			prevSet[voiceID] = struct{}{}
		}
	}
	latestMap := make(map[string]repository.OpenAITTSVoiceSnapshot, len(latest))
	for _, voice := range latest {
		voiceID := strings.TrimSpace(voice.VoiceID)
		if voiceID != "" {
			latestMap[voiceID] = voice
		}
	}
	detectedAt := time.Now().UTC()
	events := make([]model.ProviderModelChangeEvent, 0)
	for voiceID, voice := range latestMap {
		if _, ok := prevSet[voiceID]; ok {
			continue
		}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "openai",
			ChangeType: "added",
			ModelID:    voiceID,
			DetectedAt: detectedAt,
			Metadata: map[string]any{
				"trigger":     trigger,
				"name":        voice.Name,
				"language":    voice.Language,
				"description": voice.Description,
			},
		})
	}
	latestSet := make(map[string]struct{}, len(latestMap))
	for voiceID := range latestMap {
		latestSet[voiceID] = struct{}{}
	}
	for _, voiceID := range previousVoiceIDs {
		voiceID = strings.TrimSpace(voiceID)
		if voiceID == "" {
			continue
		}
		if _, ok := latestSet[voiceID]; ok {
			continue
		}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "openai",
			ChangeType: "removed",
			ModelID:    voiceID,
			DetectedAt: detectedAt,
			Metadata: map[string]any{
				"trigger": trigger,
			},
		})
	}
	if len(events) == 0 {
		return nil
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].ChangeType == events[j].ChangeType {
			return events[i].ModelID < events[j].ModelID
		}
		return events[i].ChangeType < events[j].ChangeType
	})
	return h.providerUpdateRepo.InsertChangeEvents(ctx, events)
}

func (h *OpenAITTSVoicesHandler) markProviderSnapshotFailed(ctx context.Context, previousVoiceIDs []string, errMsg string) {
	if h.providerUpdateRepo == nil {
		return
	}
	if len(previousVoiceIDs) == 0 {
		return
	}
	_ = h.providerUpdateRepo.UpsertSnapshot(ctx, "openai", previousVoiceIDs, "failed", &errMsg)
}

func openAITTSVoiceSyncRunIsStale(run *repository.OpenAITTSVoiceSyncRun, now time.Time) bool {
	if run == nil || run.Status != "running" {
		return false
	}
	reference := run.StartedAt
	if run.LastProgressAt != nil {
		reference = *run.LastProgressAt
	}
	return now.Sub(reference) > 15*time.Minute
}

func openAITTSVoiceIDs(voices []repository.OpenAITTSVoiceSnapshot) []string {
	if len(voices) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(voices))
	for _, voice := range voices {
		voiceID := strings.TrimSpace(voice.VoiceID)
		if voiceID == "" {
			continue
		}
		out = append(out, voiceID)
	}
	sort.Strings(out)
	return out
}
