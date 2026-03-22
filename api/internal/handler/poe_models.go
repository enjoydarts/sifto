package handler

import (
	"context"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type PoeModelsHandler struct {
	repo               *repository.PoeModelRepo
	service            *service.PoeCatalogService
	providerUpdateRepo *repository.ProviderModelUpdateRepo
	activeTranslations sync.Map
	processStartedAt   time.Time
}

func NewPoeModelsHandler(repo *repository.PoeModelRepo, providerUpdateRepo *repository.ProviderModelUpdateRepo, svc *service.PoeCatalogService) *PoeModelsHandler {
	return &PoeModelsHandler{
		repo:               repo,
		providerUpdateRepo: providerUpdateRepo,
		service:            svc,
		processStartedAt:   time.Now().UTC(),
	}
}

func (h *PoeModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	models, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if models == nil {
		models = make([]repository.PoeModelSnapshot, 0)
	}
	latestRun = h.resumeTranslationIfNeeded(latestRun, models)
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "poe")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                models,
		"latest_change_summary": latestChangeSummary,
	})
}

func (h *PoeModelsHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestManualRunningSyncRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if run != nil {
		models, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if latestRun != nil && latestRun.ID == run.ID {
			run = h.resumeTranslationIfNeeded(run, models)
		}
	}
	writeJSON(w, map[string]any{"run": run})
}

func (h *PoeModelsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if run, err := h.repo.GetLatestManualRunningSyncRun(r.Context()); err == nil && poeSyncRunIsStale(run, time.Now().UTC()) {
		h.failSyncRun(run.ID, "Poe description translation interrupted by local restart")
	}
	syncRunID, err := h.repo.StartSyncRun(r.Context(), "manual")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	fetchedAt := time.Now().UTC()
	models, fetchErr := h.service.FetchModels(r.Context(), strings.TrimSpace(os.Getenv("POE_API_KEY")))
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
	total, completed := poeTranslationProgress(models)
	if err := h.repo.UpdateTranslationProgress(r.Context(), syncRunID, total, completed); err != nil {
		writeRepoError(w, err)
		return
	}
	service.SetDynamicChatModelsForProvider("poe", service.PoeSnapshotsToCatalogModels(models))
	latest, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if latest == nil {
		latest = make([]repository.PoeModelSnapshot, 0)
	}
	prevModels, err := h.repo.ListPreviousSuccessfulSnapshots(r.Context(), latestRun.ID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.providerUpdateRepo != nil {
		if err := h.insertPoeChangeEvents(r.Context(), latestRun.TriggerType, prevModels, latest); err != nil {
			writeRepoError(w, err)
			return
		}
	}
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "poe")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                latest,
		"latest_change_summary": latestChangeSummary,
	})

	h.startTranslation(syncRunID, latest)
}

func (h *PoeModelsHandler) translateDescriptions(syncRunID string, models []repository.PoeModelSnapshot) {
	defer h.activeTranslations.Delete(syncRunID)
	defer func() {
		if recovered := recover(); recovered != nil {
			msg := "Poe description translation panicked"
			log.Printf("poe description translation panic sync_run_id=%s panic=%v", syncRunID, recovered)
			h.failSyncRun(syncRunID, msg)
		}
	}()
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		h.finishSyncRun(syncRunID, len(models), len(models), nil)
		return
	}
	ctx := context.Background()
	openAI := service.NewOpenAIClient()
	total, completed := poeTranslationProgress(models)
	pending := poePendingTranslationModels(models)
	for _, pendingModel := range pending {
		translateCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		translated, err := openAI.TranslateTextsToJA(translateCtx, apiKey, service.OpenRouterDescriptionTranslationModel(), map[string]string{
			pendingModel.ModelID: strings.TrimSpace(derefOptionalString(pendingModel.DescriptionEN)),
		})
		cancel()
		if err != nil {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, pendingModel.ModelID, err.Error())
			completed++
			_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
			continue
		}
		ja := strings.TrimSpace(translated[pendingModel.ModelID])
		descEN := strings.TrimSpace(derefOptionalString(pendingModel.DescriptionEN))
		if ja == "" || ja == descEN {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, pendingModel.ModelID, "translation unavailable")
			completed++
			_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
			continue
		}
		if err := h.repo.UpdateDescriptionsJA(ctx, syncRunID, map[string]string{pendingModel.ModelID: ja}); err != nil {
			msg := err.Error()
			h.finishSyncRun(syncRunID, len(models), len(models), &msg)
			return
		}
		for i := range models {
			if models[i].ModelID == pendingModel.ModelID {
				models[i].DescriptionJA = &ja
				break
			}
		}
		completed++
		_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
	}
	service.SetDynamicChatModelsForProvider("poe", service.PoeSnapshotsToCatalogModels(models))
	h.finishSyncRun(syncRunID, len(models), len(models), nil)
}

func (h *PoeModelsHandler) startTranslation(syncRunID string, models []repository.PoeModelSnapshot) {
	if _, loaded := h.activeTranslations.LoadOrStore(syncRunID, struct{}{}); loaded {
		return
	}
	go h.translateDescriptions(syncRunID, models)
}

func (h *PoeModelsHandler) resumeTranslationIfNeeded(run *repository.PoeSyncRun, models []repository.PoeModelSnapshot) *repository.PoeSyncRun {
	if h.shouldResumeAfterProcessRestart(run, models) {
		h.startTranslation(run.ID, models)
		return run
	}
	if !poeSyncRunIsStale(run, time.Now().UTC()) {
		return run
	}
	if len(poePendingTranslationModels(models)) == 0 {
		h.finishSyncRun(run.ID, len(models), len(models), nil)
		finishedAt := time.Now().UTC()
		run.FinishedAt = &finishedAt
		run.LastProgressAt = &finishedAt
		run.Status = "success"
		run.ErrorMessage = nil
		return run
	}
	h.startTranslation(run.ID, models)
	return run
}

func (h *PoeModelsHandler) shouldResumeAfterProcessRestart(run *repository.PoeSyncRun, models []repository.PoeModelSnapshot) bool {
	if run == nil || run.Status != "running" {
		return false
	}
	if len(poePendingTranslationModels(models)) == 0 {
		return false
	}
	if _, active := h.activeTranslations.Load(run.ID); active {
		return false
	}
	ref := run.StartedAt
	if run.LastProgressAt != nil {
		ref = *run.LastProgressAt
	}
	return ref.Before(h.processStartedAt)
}

func (h *PoeModelsHandler) finishSyncRun(syncRunID string, fetchedCount, acceptedCount int, errMsg *string) {
	finishCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := h.repo.FinishSyncRun(finishCtx, syncRunID, fetchedCount, acceptedCount, errMsg); err != nil {
		log.Printf("poe sync finish failed sync_run_id=%s err=%v", syncRunID, err)
	}
}

func (h *PoeModelsHandler) failSyncRun(syncRunID, errMsg string) {
	failCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := h.repo.FailSyncRun(failCtx, syncRunID, errMsg); err != nil {
		log.Printf("poe sync fail update failed sync_run_id=%s err=%v", syncRunID, err)
	}
}

func poeSyncRunIsStale(run *repository.PoeSyncRun, now time.Time) bool {
	if run == nil || run.Status != "running" {
		return false
	}
	ref := run.StartedAt
	if run.LastProgressAt != nil {
		ref = *run.LastProgressAt
	}
	return now.Sub(ref) > 2*time.Minute
}

func poeTranslationProgress(models []repository.PoeModelSnapshot) (total int, completed int) {
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

func poePendingTranslationModels(models []repository.PoeModelSnapshot) []repository.PoeModelSnapshot {
	pending := make([]repository.PoeModelSnapshot, 0)
	for _, model := range models {
		descEN := strings.TrimSpace(derefOptionalString(model.DescriptionEN))
		if descEN == "" {
			continue
		}
		descJA := strings.TrimSpace(derefOptionalString(model.DescriptionJA))
		if descJA != "" && descJA != descEN {
			continue
		}
		pending = append(pending, model)
	}
	return pending
}

func buildPoeRecentChanges(previous, current []repository.PoeModelSnapshot) map[string]string {
	prevMap := make(map[string]struct{}, len(previous))
	currMap := make(map[string]struct{}, len(current))
	changes := make(map[string]string)
	for _, item := range previous {
		prevMap[item.ModelID] = struct{}{}
	}
	for _, item := range current {
		currMap[item.ModelID] = struct{}{}
		if _, existed := prevMap[item.ModelID]; !existed {
			changes[item.ModelID] = "added"
		}
	}
	for _, item := range previous {
		if _, exists := currMap[item.ModelID]; !exists {
			changes[item.ModelID] = "removed"
		}
	}
	return changes
}

func (h *PoeModelsHandler) insertPoeChangeEvents(ctx context.Context, trigger string, previous, current []repository.PoeModelSnapshot) error {
	if h.providerUpdateRepo == nil {
		return nil
	}
	changes := buildPoeRecentChanges(previous, current)
	if len(changes) == 0 {
		return nil
	}
	added := make([]string, 0)
	removed := make([]string, 0)
	for modelID, change := range changes {
		switch change {
		case "added":
			added = append(added, modelID)
		case "removed":
			removed = append(removed, modelID)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	now := time.Now().UTC()
	events := make([]model.ProviderModelChangeEvent, 0, len(added)+len(removed))
	for _, modelID := range added {
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "poe",
			ChangeType: "added",
			ModelID:    modelID,
			DetectedAt: now,
			Metadata:   map[string]any{"source": "poe_sync", "trigger": trigger},
		})
	}
	for _, modelID := range removed {
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "poe",
			ChangeType: "removed",
			ModelID:    modelID,
			DetectedAt: now,
			Metadata:   map[string]any{"source": "poe_sync", "trigger": trigger},
		})
	}
	return h.providerUpdateRepo.InsertChangeEvents(ctx, events)
}
