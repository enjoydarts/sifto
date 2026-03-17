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

func (h *OpenRouterModelsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	syncRunID, err := h.repo.StartSyncRun(r.Context())
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
			continue
		}
		models[i].DescriptionJA = &ja
		if err := h.repo.UpdateDescriptionsJA(ctx, syncRunID, map[string]string{models[i].ModelID: ja}); err != nil {
			log.Printf("openrouter description translation update failed sync_run_id=%s model_id=%s err=%v", syncRunID, models[i].ModelID, err)
			return
		}
	}
	service.SetDynamicChatModels(service.OpenRouterSnapshotsToCatalogModels(models))
}

func derefOptionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
