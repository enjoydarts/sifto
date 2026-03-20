package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type openRouterModelListEntry struct {
	repository.OpenRouterModelSnapshot
	Availability    string  `json:"availability"`
	RawAvailability string  `json:"raw_availability"`
	Reason          *string `json:"reason,omitempty"`
	RecentChange    *string `json:"recent_change,omitempty"`
	OverrideEnabled bool    `json:"override_enabled"`
}

type OpenRouterModelsHandler struct {
	repo               *repository.OpenRouterModelRepo
	overrideRepo       *repository.OpenRouterModelOverrideRepo
	service            *service.OpenRouterCatalogService
	providerUpdateRepo *repository.ProviderModelUpdateRepo
	cache              service.JSONCache
}

func NewOpenRouterModelsHandler(repo *repository.OpenRouterModelRepo, overrideRepo *repository.OpenRouterModelOverrideRepo, providerUpdateRepo *repository.ProviderModelUpdateRepo, svc *service.OpenRouterCatalogService, cache service.JSONCache) *OpenRouterModelsHandler {
	return &OpenRouterModelsHandler{repo: repo, overrideRepo: overrideRepo, providerUpdateRepo: providerUpdateRepo, service: svc, cache: cache}
}

func (h *OpenRouterModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
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
	recentChanges := map[string]string(nil)
	if latestRun != nil && latestRun.TriggerType == "manual" {
		recentChanges = buildOpenRouterRecentChanges(prevModels, models)
	}
	overrides, err := h.listOverrides(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	available, unavailable := splitOpenRouterModelEntries(models, prevModels, recentChanges, overrides)
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "openrouter")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                available,
		"unavailable_models":    unavailable,
		"latest_change_summary": latestChangeSummary,
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
	userID := middleware.GetUserID(r)
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
	if h.providerUpdateRepo != nil {
		if err := h.insertOpenRouterChangeEvents(r.Context(), latestRun.TriggerType, prevModels, latest); err != nil {
			writeRepoError(w, err)
			return
		}
	}
	recentChanges := map[string]string(nil)
	if latestRun != nil && latestRun.TriggerType == "manual" {
		recentChanges = buildOpenRouterRecentChanges(prevModels, latest)
	}
	overrides, err := h.listOverrides(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	available, unavailable := splitOpenRouterModelEntries(latest, prevModels, recentChanges, overrides)
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "openrouter")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                available,
		"unavailable_models":    unavailable,
		"latest_change_summary": latestChangeSummary,
	})

	go h.translateDescriptions(syncRunID, models)
}

func (h *OpenRouterModelsHandler) UpdateStructuredOutputOverride(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		ModelID               string `json:"model_id"`
		AllowStructuredOutput bool   `json:"allow_structured_output"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	modelID := strings.TrimSpace(body.ModelID)
	if modelID == "" {
		http.Error(w, "model_id is required", http.StatusBadRequest)
		return
	}
	if h.overrideRepo == nil {
		http.Error(w, "override repository is not configured", http.StatusInternalServerError)
		return
	}
	if !body.AllowStructuredOutput {
		if err := h.overrideRepo.Delete(r.Context(), userID, modelID); err != nil {
			writeRepoError(w, err)
			return
		}
		h.bumpUserSettingsVersion(r.Context(), userID)
		writeJSON(w, map[string]any{
			"model_id":                modelID,
			"override_enabled":        false,
			"allow_structured_output": false,
		})
		return
	}
	latest, _, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var snapshot *repository.OpenRouterModelSnapshot
	for i := range latest {
		if latest[i].ModelID == modelID {
			snapshot = &latest[i]
			break
		}
	}
	if snapshot == nil {
		http.Error(w, "removed models cannot be overridden", http.StatusBadRequest)
		return
	}
	rawAvailability, _ := service.OpenRouterSnapshotAvailability(*snapshot)
	if rawAvailability != service.OpenRouterModelConstrained {
		http.Error(w, "only constrained models can be overridden", http.StatusBadRequest)
		return
	}
	record, err := h.overrideRepo.Upsert(r.Context(), userID, modelID, true)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.bumpUserSettingsVersion(r.Context(), userID)
	writeJSON(w, map[string]any{
		"model_id":                record.ModelID,
		"override_enabled":        true,
		"allow_structured_output": true,
	})
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
	pending := openRouterPendingTranslationModels(models)
	for _, pendingModel := range pending {
		enriched := service.EnrichOpenRouterDescriptionsJA(ctx, h.repo, openAI, []repository.OpenRouterModelSnapshot{pendingModel})
		if len(enriched) == 0 {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, pendingModel.ModelID, "empty translation response")
			continue
		}
		ja := strings.TrimSpace(derefOptionalString(enriched[0].DescriptionJA))
		descEN := strings.TrimSpace(derefOptionalString(pendingModel.DescriptionEN))
		if ja == "" || ja == descEN {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, pendingModel.ModelID, "translation unavailable")
			completed++
			_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
			continue
		}
		for i := range models {
			if models[i].ModelID == pendingModel.ModelID {
				models[i].DescriptionJA = &ja
				break
			}
		}
		if err := h.repo.UpdateDescriptionsJA(ctx, syncRunID, map[string]string{pendingModel.ModelID: ja}); err != nil {
			log.Printf("openrouter description translation update failed sync_run_id=%s model_id=%s err=%v", syncRunID, pendingModel.ModelID, err)
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

func openRouterPendingTranslationModels(models []repository.OpenRouterModelSnapshot) []repository.OpenRouterModelSnapshot {
	pending := make([]repository.OpenRouterModelSnapshot, 0)
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

func (h *OpenRouterModelsHandler) bumpUserSettingsVersion(ctx context.Context, userID string) {
	if h.cache == nil || strings.TrimSpace(userID) == "" {
		return
	}
	if _, err := h.cache.BumpVersion(ctx, cacheVersionKeyUserSettings(userID)); err != nil {
		log.Printf("openrouter override settings cache bump failed user_id=%s err=%v", userID, err)
	}
}

func (h *OpenRouterModelsHandler) listOverrides(ctx context.Context, userID string) (map[string]repository.OpenRouterModelOverride, error) {
	if h.overrideRepo == nil || strings.TrimSpace(userID) == "" {
		return map[string]repository.OpenRouterModelOverride{}, nil
	}
	return h.overrideRepo.ListByUser(ctx, userID)
}

func splitOpenRouterModelEntries(current, previous []repository.OpenRouterModelSnapshot, recentChanges map[string]string, overrides map[string]repository.OpenRouterModelOverride) ([]openRouterModelListEntry, []openRouterModelListEntry) {
	available := make([]openRouterModelListEntry, 0, len(current))
	unavailable := make([]openRouterModelListEntry, 0)
	currentMap := make(map[string]repository.OpenRouterModelSnapshot, len(current))
	for _, model := range current {
		currentMap[model.ModelID] = model
		rawAvailability, _ := service.OpenRouterSnapshotAvailability(model)
		overrideEnabled := false
		if override, ok := overrides[model.ModelID]; ok && override.AllowStructuredOutput {
			overrideEnabled = true
		}
		availability, reason := service.OpenRouterEffectiveAvailability(model, overrideEnabled, false)
		entry := openRouterModelListEntry{
			OpenRouterModelSnapshot: model,
			Availability:            string(availability),
			RawAvailability:         string(rawAvailability),
			OverrideEnabled:         overrideEnabled && rawAvailability == service.OpenRouterModelConstrained,
		}
		if reason != "" {
			r := reason
			entry.Reason = &r
		}
		if recentChange, ok := recentChanges[model.ModelID]; ok {
			rc := recentChange
			entry.RecentChange = &rc
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
		entry := openRouterModelListEntry{
			OpenRouterModelSnapshot: model,
			Availability:            string(service.OpenRouterModelRemoved),
			RawAvailability:         string(service.OpenRouterModelRemoved),
			Reason:                  &reason,
		}
		if recentChange, ok := recentChanges[model.ModelID]; ok {
			rc := recentChange
			entry.RecentChange = &rc
		}
		unavailable = append(unavailable, entry)
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

func buildOpenRouterRecentChanges(previous, current []repository.OpenRouterModelSnapshot) map[string]string {
	prevMap := make(map[string]service.OpenRouterModelAvailability, len(previous))
	for _, item := range previous {
		state, _ := service.OpenRouterSnapshotAvailability(item)
		prevMap[item.ModelID] = state
	}
	currMap := make(map[string]service.OpenRouterModelAvailability, len(current))
	changes := make(map[string]string)
	for _, item := range current {
		state, _ := service.OpenRouterSnapshotAvailability(item)
		currMap[item.ModelID] = state
		prevState, existed := prevMap[item.ModelID]
		if !existed {
			changes[item.ModelID] = string(service.OpenRouterModelAvailable)
			continue
		}
		if prevState == service.OpenRouterModelAvailable && state == service.OpenRouterModelConstrained {
			changes[item.ModelID] = string(service.OpenRouterModelConstrained)
		}
	}
	for _, item := range previous {
		if _, exists := currMap[item.ModelID]; !exists {
			changes[item.ModelID] = string(service.OpenRouterModelRemoved)
		}
	}
	return changes
}

func (h *OpenRouterModelsHandler) insertOpenRouterChangeEvents(ctx context.Context, trigger string, previous, current []repository.OpenRouterModelSnapshot) error {
	if h.providerUpdateRepo == nil {
		return nil
	}
	added, constrained, removed := diffOpenRouterChangeSets(previous, current)
	if len(added) == 0 && len(constrained) == 0 && len(removed) == 0 {
		return nil
	}
	now := time.Now().UTC()
	events := make([]model.ProviderModelChangeEvent, 0, len(added)+len(constrained)+len(removed))
	for _, modelID := range added {
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "openrouter",
			ChangeType: "added",
			ModelID:    modelID,
			DetectedAt: now,
			Metadata:   map[string]any{"source": "openrouter_sync", "trigger": trigger},
		})
	}
	for _, modelID := range constrained {
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "openrouter",
			ChangeType: "constrained",
			ModelID:    modelID,
			DetectedAt: now,
			Metadata:   map[string]any{"source": "openrouter_sync", "trigger": trigger},
		})
	}
	for _, modelID := range removed {
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "openrouter",
			ChangeType: "removed",
			ModelID:    modelID,
			DetectedAt: now,
			Metadata:   map[string]any{"source": "openrouter_sync", "trigger": trigger},
		})
	}
	return h.providerUpdateRepo.InsertChangeEvents(ctx, events)
}

func diffOpenRouterChangeSets(previous, current []repository.OpenRouterModelSnapshot) (added, constrained, removed []string) {
	changes := buildOpenRouterRecentChanges(previous, current)
	for modelID, change := range changes {
		switch change {
		case string(service.OpenRouterModelAvailable):
			added = append(added, modelID)
		case string(service.OpenRouterModelConstrained):
			constrained = append(constrained, modelID)
		case string(service.OpenRouterModelRemoved):
			removed = append(removed, modelID)
		}
	}
	sort.Strings(added)
	sort.Strings(constrained)
	sort.Strings(removed)
	return added, constrained, removed
}
