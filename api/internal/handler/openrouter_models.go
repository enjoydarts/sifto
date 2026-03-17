package handler

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

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
	writeJSON(w, map[string]any{
		"latest_run": latestRun,
		"models":     models,
	})
}

func (h *OpenRouterModelsHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestManualRunningSyncRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
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
	writeJSON(w, map[string]any{
		"latest_run": latestRun,
		"models":     latest,
	})

	go h.translateDescriptions(syncRunID, models)
}

func (h *OpenRouterModelsHandler) translateDescriptions(syncRunID string, models []repository.OpenRouterModelSnapshot) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	openAI := service.NewOpenAIClient()
	total, completed := openRouterTranslationProgress(models)
	for i := range models {
		descEN := strings.TrimSpace(derefOptionalString(models[i].DescriptionEN))
		if descEN == "" {
			continue
		}
		enriched := service.EnrichOpenRouterDescriptionsJA(ctx, h.repo, openAI, []repository.OpenRouterModelSnapshot{models[i]})
		if len(enriched) == 0 {
			continue
		}
		ja := strings.TrimSpace(derefOptionalString(enriched[0].DescriptionJA))
		if ja == "" || ja == descEN {
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
