package handler

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type openRouterModelListEntry struct {
	repository.OpenRouterModelSnapshot
	Availability string  `json:"availability"`
	Reason       *string `json:"reason,omitempty"`
}

type OpenRouterModelsHandler struct {
	repo    *repository.OpenRouterModelRepo
	service *service.OpenRouterCatalogService
}

func NewOpenRouterModelsHandler(repo *repository.OpenRouterModelRepo, svc *service.OpenRouterCatalogService) *OpenRouterModelsHandler {
	return &OpenRouterModelsHandler{repo: repo, service: svc}
}

func (h *OpenRouterModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	models, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	prevModels := []repository.OpenRouterModelSnapshot{}
	if latestRun != nil {
		prevModels, err = h.repo.ListPreviousSuccessfulSnapshots(r.Context(), latestRun.ID)
		if err != nil {
			writeRepoError(w, err)
			return
		}
	}
	available, unavailable := splitOpenRouterModelEntries(models, prevModels)
	writeJSON(w, map[string]any{
		"latest_run":         latestRun,
		"models":             available,
		"unavailable_models": unavailable,
	})
}

func (h *OpenRouterModelsHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestManualRunningSyncRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if openRouterSyncRunIsStale(run, time.Now().UTC()) {
		if err := h.repo.FailSyncRun(r.Context(), run.ID, "OpenRouter description translation stalled"); err != nil {
			writeRepoError(w, err)
			return
		}
		run = nil
	}
	writeJSON(w, map[string]any{"run": run})
}

func (h *OpenRouterModelsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	syncRunID, err := h.repo.StartSyncRun(r.Context(), "manual")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	fetchedAt := time.Now().UTC()
	models, fetchErr := h.service.FetchTextGenerationModels(r.Context())
	if fetchErr != nil {
		msg := fetchErr.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, 0, 0, &msg)
		http.Error(w, fetchErr.Error(), http.StatusBadGateway)
		return
	}
	if cache, err := h.repo.ListLatestDescriptionCache(r.Context()); err == nil {
		models, _ = service.ApplyOpenRouterDescriptionCache(models, cache)
	}
	if err := h.repo.InsertSnapshots(r.Context(), syncRunID, fetchedAt, models); err != nil {
		msg := err.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(models), 0, &msg)
		writeRepoError(w, err)
		return
	}
	total, completed := openRouterTranslationProgress(models)
	if err := h.repo.UpdateTranslationProgress(r.Context(), syncRunID, total, completed); err != nil {
		writeRepoError(w, err)
		return
	}
	service.SetDynamicChatModels(service.OpenRouterSnapshotsToCatalogModels(models))
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
	available, unavailable := splitOpenRouterModelEntries(latest, prevModels)
	writeJSON(w, map[string]any{
		"latest_run":         latestRun,
		"models":             available,
		"unavailable_models": unavailable,
	})

	go h.translateDescriptions(syncRunID, models)
}

func (h *OpenRouterModelsHandler) translateDescriptions(syncRunID string, models []repository.OpenRouterModelSnapshot) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			msg := "OpenRouter description translation panicked"
			log.Printf("openrouter description translation panic sync_run_id=%s panic=%v", syncRunID, recovered)
			_ = h.repo.FailSyncRun(context.Background(), syncRunID, msg)
		}
	}()
	openAI := service.NewOpenAIClient()
	total, completed := openRouterTranslationProgress(models)
	for i := range models {
		descEN := strings.TrimSpace(derefOptionalString(models[i].DescriptionEN))
		if descEN == "" {
			continue
		}
		enriched := service.EnrichOpenRouterDescriptionsJA(ctx, h.repo, openAI, []repository.OpenRouterModelSnapshot{models[i]})
		if len(enriched) == 0 {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, models[i].ModelID, "empty translation response")
			continue
		}
		ja := strings.TrimSpace(derefOptionalString(enriched[0].DescriptionJA))
		if ja == "" || ja == descEN {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, models[i].ModelID, "translation unavailable")
			completed++
			_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
			continue
		}
		models[i].DescriptionJA = &ja
		if err := h.repo.UpdateDescriptionsJA(ctx, syncRunID, map[string]string{models[i].ModelID: ja}); err != nil {
			log.Printf("openrouter description translation update failed sync_run_id=%s model_id=%s err=%v", syncRunID, models[i].ModelID, err)
			msg := err.Error()
			_ = h.repo.FinishSyncRun(ctx, syncRunID, len(models), len(models), &msg)
			return
		}
		completed++
		_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
	}
	service.SetDynamicChatModels(service.OpenRouterSnapshotsToCatalogModels(models))
	if err := h.repo.FinishSyncRun(ctx, syncRunID, len(models), len(models), nil); err != nil {
		log.Printf("openrouter sync finish failed sync_run_id=%s err=%v", syncRunID, err)
	}
}

func derefOptionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func openRouterSyncRunIsStale(run *repository.OpenRouterSyncRun, now time.Time) bool {
	if run == nil || run.Status != "running" {
		return false
	}
	ref := run.StartedAt
	if run.LastProgressAt != nil {
		ref = *run.LastProgressAt
	}
	return now.Sub(ref) > 2*time.Minute
}

func openRouterTranslationProgress(models []repository.OpenRouterModelSnapshot) (total int, completed int) {
	for _, model := range models {
		descEN := strings.TrimSpace(derefOptionalString(model.DescriptionEN))
		if descEN == "" {
			continue
		}
		total++
		descJA := strings.TrimSpace(derefOptionalString(model.DescriptionJA))
		if descJA != "" && descJA != descEN {
			completed++
		}
	}
	return total, completed
}

func splitOpenRouterModelEntries(current, previous []repository.OpenRouterModelSnapshot) ([]openRouterModelListEntry, []openRouterModelListEntry) {
	available := make([]openRouterModelListEntry, 0, len(current))
	unavailable := make([]openRouterModelListEntry, 0)
	currentMap := make(map[string]repository.OpenRouterModelSnapshot, len(current))
	for _, model := range current {
		currentMap[model.ModelID] = model
		availability, reason := service.OpenRouterSnapshotAvailability(model)
		entry := openRouterModelListEntry{
			OpenRouterModelSnapshot: model,
			Availability:            string(availability),
		}
		if reason != "" {
			r := reason
			entry.Reason = &r
		}
		if availability == service.OpenRouterModelAvailable {
			available = append(available, entry)
		} else {
			unavailable = append(unavailable, entry)
		}
	}
	for _, model := range previous {
		if _, exists := currentMap[model.ModelID]; exists {
			continue
		}
		reason := "removed"
		unavailable = append(unavailable, openRouterModelListEntry{
			OpenRouterModelSnapshot: model,
			Availability:            string(service.OpenRouterModelRemoved),
			Reason:                  &reason,
		})
	}
	sort.Slice(available, func(i, j int) bool {
		if available[i].ProviderSlug == available[j].ProviderSlug {
			return available[i].DisplayName < available[j].DisplayName
		}
		return available[i].ProviderSlug < available[j].ProviderSlug
	})
	sort.Slice(unavailable, func(i, j int) bool {
		if unavailable[i].Availability == unavailable[j].Availability {
			if unavailable[i].ProviderSlug == unavailable[j].ProviderSlug {
				return unavailable[i].DisplayName < unavailable[j].DisplayName
			}
			return unavailable[i].ProviderSlug < unavailable[j].ProviderSlug
		}
		return unavailable[i].Availability < unavailable[j].Availability
	})
	return available, unavailable
}
