package handler

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type AivisModelsHandler struct {
	repo               *repository.AivisModelRepo
	service            *service.AivisCatalogService
	providerUpdateRepo *repository.ProviderModelUpdateRepo
}

func NewAivisModelsHandler(repo *repository.AivisModelRepo, providerUpdateRepo *repository.ProviderModelUpdateRepo, svc *service.AivisCatalogService) *AivisModelsHandler {
	return &AivisModelsHandler{repo: repo, service: svc, providerUpdateRepo: providerUpdateRepo}
}

func (h *AivisModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	models, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	removedModels := make([]repository.AivisModelSnapshot, 0)
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "aivis")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
		if summary != nil && len(summary.Removed) > 0 && latestRun != nil {
			prevModels, err := h.repo.ListPreviousSuccessfulSnapshots(r.Context(), latestRun.ID)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			removedModels = filterAivisRemovedModels(prevModels, summary.Removed)
		}
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                models,
		"removed_models":        removedModels,
		"latest_change_summary": latestChangeSummary,
	})
}

func (h *AivisModelsHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestManualRunningSyncRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if aivisSyncRunIsStale(run, time.Now().UTC()) {
		if err := h.repo.FailSyncRun(r.Context(), run.ID, "Aivis model sync stalled"); err != nil {
			writeRepoError(w, err)
			return
		}
		run = nil
	}
	writeJSON(w, map[string]any{"run": run})
}

func (h *AivisModelsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if run, err := h.repo.GetLatestManualRunningSyncRun(r.Context()); err == nil && run != nil {
		if aivisSyncRunIsStale(run, time.Now().UTC()) {
			if err := h.repo.FailSyncRun(r.Context(), run.ID, "Aivis model sync interrupted"); err != nil {
				writeRepoError(w, err)
				return
			}
		} else {
			http.Error(w, "aivis model sync already running", http.StatusConflict)
			return
		}
	}
	syncRunID, err := h.repo.StartSyncRun(r.Context(), "manual")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	models, fetchErr := h.service.FetchModels(r.Context())
	if fetchErr != nil {
		msg := fetchErr.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, 0, 0, &msg)
		if h.providerUpdateRepo != nil {
			_ = h.providerUpdateRepo.UpsertSnapshot(r.Context(), "aivis", []string{}, "failed", &msg)
		}
		http.Error(w, fetchErr.Error(), http.StatusBadGateway)
		return
	}
	fetchedAt := time.Now().UTC()
	if err := h.repo.InsertSnapshots(r.Context(), syncRunID, fetchedAt, models); err != nil {
		msg := err.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(models), 0, &msg)
		writeRepoError(w, err)
		return
	}
	if err := h.repo.FinishSyncRun(r.Context(), syncRunID, len(models), len(models), nil); err != nil {
		writeRepoError(w, err)
		return
	}
	if h.providerUpdateRepo != nil {
		modelIDs := make([]string, 0, len(models))
		for _, item := range models {
			modelIDs = append(modelIDs, item.AivmModelUUID)
		}
		if err := h.providerUpdateRepo.UpsertSnapshot(r.Context(), "aivis", modelIDs, "ok", nil); err != nil {
			writeRepoError(w, err)
			return
		}
		latest, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
		if err != nil {
			writeRepoError(w, err)
			return
		}
		prevModels, err := h.repo.ListPreviousSuccessfulSnapshots(r.Context(), latestRun.ID)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if err := h.insertAivisChangeEvents(r.Context(), latestRun.TriggerType, prevModels, latest); err != nil {
			writeRepoError(w, err)
			return
		}
	}
	h.List(w, r)
}

func aivisSyncRunIsStale(run *repository.AivisSyncRun, now time.Time) bool {
	if run == nil || run.Status != "running" {
		return false
	}
	reference := run.StartedAt
	if run.LastProgressAt != nil {
		reference = *run.LastProgressAt
	}
	return now.Sub(reference) > 15*time.Minute
}

func filterAivisRemovedModels(previous []repository.AivisModelSnapshot, removed []model.ProviderModelChangeEvent) []repository.AivisModelSnapshot {
	if len(previous) == 0 || len(removed) == 0 {
		return make([]repository.AivisModelSnapshot, 0)
	}
	removedIDs := make(map[string]struct{}, len(removed))
	for _, ev := range removed {
		modelID := strings.TrimSpace(ev.ModelID)
		if modelID != "" {
			removedIDs[modelID] = struct{}{}
		}
	}
	out := make([]repository.AivisModelSnapshot, 0, len(removedIDs))
	for _, snapshot := range previous {
		if _, ok := removedIDs[snapshot.AivmModelUUID]; ok {
			out = append(out, snapshot)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalDownloadCount == out[j].TotalDownloadCount {
			return out[i].Name < out[j].Name
		}
		return out[i].TotalDownloadCount > out[j].TotalDownloadCount
	})
	return out
}

func (h *AivisModelsHandler) insertAivisChangeEvents(ctx context.Context, trigger string, previous, latest []repository.AivisModelSnapshot) error {
	prevMap := make(map[string]repository.AivisModelSnapshot, len(previous))
	for _, snapshot := range previous {
		prevMap[snapshot.AivmModelUUID] = snapshot
	}
	latestMap := make(map[string]repository.AivisModelSnapshot, len(latest))
	for _, snapshot := range latest {
		latestMap[snapshot.AivmModelUUID] = snapshot
	}
	detectedAt := time.Now().UTC()
	events := make([]model.ProviderModelChangeEvent, 0)
	for modelID, snapshot := range latestMap {
		if _, ok := prevMap[modelID]; ok {
			continue
		}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "aivis",
			ChangeType: "added",
			ModelID:    modelID,
			DetectedAt: detectedAt,
			Metadata: map[string]any{
				"trigger":        trigger,
				"name":           snapshot.Name,
				"category":       snapshot.Category,
				"voice_timbre":   snapshot.VoiceTimbre,
				"download_count": snapshot.TotalDownloadCount,
			},
		})
	}
	for modelID, snapshot := range prevMap {
		if _, ok := latestMap[modelID]; ok {
			continue
		}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "aivis",
			ChangeType: "removed",
			ModelID:    modelID,
			DetectedAt: detectedAt,
			Metadata: map[string]any{
				"trigger":        trigger,
				"name":           snapshot.Name,
				"category":       snapshot.Category,
				"voice_timbre":   snapshot.VoiceTimbre,
				"download_count": snapshot.TotalDownloadCount,
			},
		})
	}
	if len(events) == 0 || h.providerUpdateRepo == nil {
		return nil
	}
	return h.providerUpdateRepo.InsertChangeEvents(ctx, events)
}
