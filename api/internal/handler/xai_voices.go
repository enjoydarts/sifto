package handler

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type xaiVoiceCatalogFetcher interface {
	FetchVoices(ctx context.Context, apiKey string) ([]repository.XAIVoiceSnapshot, error)
}

type xaiVoiceSettingsRepo interface {
	GetXAIAPIKeyEncrypted(ctx context.Context, userID string) (*string, error)
}

type XAIVoicesHandler struct {
	repo               *repository.XAIVoiceRepo
	settingsRepo       xaiVoiceSettingsRepo
	providerUpdateRepo *repository.ProviderModelUpdateRepo
	cipher             *service.SecretCipher
	service            xaiVoiceCatalogFetcher
}

func NewXAIVoicesHandler(repo *repository.XAIVoiceRepo, settingsRepo xaiVoiceSettingsRepo, providerUpdateRepo *repository.ProviderModelUpdateRepo, cipher *service.SecretCipher, svc xaiVoiceCatalogFetcher) *XAIVoicesHandler {
	return &XAIVoicesHandler{
		repo:               repo,
		settingsRepo:       settingsRepo,
		providerUpdateRepo: providerUpdateRepo,
		cipher:             cipher,
		service:            svc,
	}
}

func (h *XAIVoicesHandler) List(w http.ResponseWriter, r *http.Request) {
	latestRun, err := h.repo.GetLatestRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	voices, _, err := h.repo.ListLatestSuccessfulSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "xai")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	if voices == nil {
		voices = make([]repository.XAIVoiceSnapshot, 0)
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"voices":                voices,
		"latest_change_summary": latestChangeSummary,
	})
}

func (h *XAIVoicesHandler) Status(w http.ResponseWriter, r *http.Request) {
	latestRun, err := h.repo.GetLatestRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if xaiVoiceSyncRunIsStale(latestRun, time.Now().UTC()) {
		if err := h.repo.FailSyncRun(r.Context(), latestRun.ID, "xai voice sync stalled"); err != nil {
			writeRepoError(w, err)
			return
		}
		latestRun = nil
	}
	if latestRun != nil && latestRun.Status != "running" {
		latestRun = nil
	}
	writeJSON(w, map[string]any{"run": latestRun})
}

func (h *XAIVoicesHandler) Sync(w http.ResponseWriter, r *http.Request) {
	latestRun, err := h.repo.GetLatestRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if latestRun != nil && latestRun.Status == "running" {
		if xaiVoiceSyncRunIsStale(latestRun, time.Now().UTC()) {
			if err := h.repo.FailSyncRun(r.Context(), latestRun.ID, "xai voice sync interrupted"); err != nil {
				writeRepoError(w, err)
				return
			}
		} else {
			http.Error(w, "xai voice sync already running", http.StatusConflict)
			return
		}
	}

	userID := middleware.GetUserID(r)
	apiKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetXAIAPIKeyEncrypted, h.cipher, userID, "")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
		http.Error(w, "xai api key is not configured", http.StatusBadRequest)
		return
	}

	previousModelIDs, err := h.previousVoiceIDs(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}

	syncRunID, err := h.repo.StartSyncRun(r.Context(), "manual")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	voices, fetchErr := h.service.FetchVoices(r.Context(), *apiKey)
	if fetchErr != nil {
		msg := fetchErr.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, 0, 0, &msg)
		if h.providerUpdateRepo != nil {
			_ = h.providerUpdateRepo.UpsertSnapshot(r.Context(), "xai", previousModelIDs, "failed", &msg)
		}
		http.Error(w, fetchErr.Error(), http.StatusBadGateway)
		return
	}

	fetchedAt := time.Now().UTC()
	if err := h.repo.InsertSnapshots(r.Context(), syncRunID, fetchedAt, voices); err != nil {
		msg := err.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), 0, &msg)
		h.markProviderSnapshotFailed(r.Context(), previousModelIDs, msg)
		writeRepoError(w, err)
		return
	}

	if h.providerUpdateRepo != nil {
		latestModelIDs := xaiVoiceIDs(voices)
		if err := h.providerUpdateRepo.UpsertSnapshot(r.Context(), "xai", latestModelIDs, "ok", nil); err != nil {
			msg := err.Error()
			_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), len(voices), &msg)
			h.markProviderSnapshotFailed(r.Context(), previousModelIDs, msg)
			writeRepoError(w, err)
			return
		}
		if err := h.insertXAIVoiceChangeEvents(r.Context(), "manual", previousModelIDs, voices); err != nil {
			msg := err.Error()
			_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), len(voices), &msg)
			h.markProviderSnapshotFailed(r.Context(), previousModelIDs, msg)
			writeRepoError(w, err)
			return
		}
	}
	if err := h.repo.FinishSyncRun(r.Context(), syncRunID, len(voices), len(voices), nil); err != nil {
		msg := err.Error()
		h.markProviderSnapshotFailed(r.Context(), previousModelIDs, msg)
		writeRepoError(w, err)
		return
	}

	h.List(w, r)
}

func (h *XAIVoicesHandler) previousVoiceIDs(ctx context.Context) ([]string, error) {
	if h.providerUpdateRepo == nil {
		return nil, nil
	}
	snapshot, err := h.providerUpdateRepo.GetSnapshot(ctx, "xai")
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

func (h *XAIVoicesHandler) insertXAIVoiceChangeEvents(ctx context.Context, trigger string, previousModelIDs []string, latest []repository.XAIVoiceSnapshot) error {
	if h.providerUpdateRepo == nil {
		return nil
	}
	prevSet := make(map[string]struct{}, len(previousModelIDs))
	for _, modelID := range previousModelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID != "" {
			prevSet[modelID] = struct{}{}
		}
	}
	latestMap := make(map[string]repository.XAIVoiceSnapshot, len(latest))
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
			Provider:   "xai",
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
	for _, modelID := range previousModelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if _, ok := latestSet[modelID]; ok {
			continue
		}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "xai",
			ChangeType: "removed",
			ModelID:    modelID,
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

func (h *XAIVoicesHandler) markProviderSnapshotFailed(ctx context.Context, previousModelIDs []string, errMsg string) {
	if h.providerUpdateRepo == nil {
		return
	}
	if errMsg == "" {
		return
	}
	msg := errMsg
	_ = h.providerUpdateRepo.UpsertSnapshot(ctx, "xai", previousModelIDs, "failed", &msg)
}

func xaiVoiceSyncRunIsStale(run *repository.XAIVoiceSyncRun, now time.Time) bool {
	if run == nil || run.Status != "running" {
		return false
	}
	reference := run.StartedAt
	if run.LastProgressAt != nil {
		reference = *run.LastProgressAt
	}
	return now.Sub(reference) > 15*time.Minute
}

func xaiVoiceIDs(voices []repository.XAIVoiceSnapshot) []string {
	out := make([]string, 0, len(voices))
	for _, voice := range voices {
		voiceID := strings.TrimSpace(voice.VoiceID)
		if voiceID != "" {
			out = append(out, voiceID)
		}
	}
	sort.Strings(out)
	return out
}
