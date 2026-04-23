package handler

import (
	"context"
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

type DeepInfraModelsHandler struct {
	repo               *repository.DeepInfraModelRepo
	settingsRepo       *repository.UserSettingsRepo
	service            *service.DeepInfraCatalogService
	cipher             *service.SecretCipher
	providerUpdateRepo *repository.ProviderModelUpdateRepo
}

type deepInfraModelsResponseEntry struct {
	ModelID             string    `json:"model_id"`
	DisplayName         string    `json:"display_name"`
	ProviderSlug        string    `json:"provider_slug"`
	ModelType           string    `json:"model_type,omitempty"`
	DescriptionEN       *string   `json:"description_en,omitempty"`
	DescriptionJA       *string   `json:"description_ja,omitempty"`
	ContextLength       *int      `json:"context_length,omitempty"`
	MaxTokens           *int      `json:"max_tokens,omitempty"`
	InputPerMTokUSD     *float64  `json:"input_per_mtok_usd,omitempty"`
	OutputPerMTokUSD    *float64  `json:"output_per_mtok_usd,omitempty"`
	CacheReadPerMTokUSD *float64  `json:"cache_read_per_mtok_usd,omitempty"`
	FetchedAt           time.Time `json:"fetched_at"`
}

type deepInfraModelListEntry struct {
	deepInfraModelsResponseEntry
	Availability string  `json:"availability"`
	RecentChange *string `json:"recent_change,omitempty"`
}

func NewDeepInfraModelsHandler(repo *repository.DeepInfraModelRepo, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, providerUpdateRepo *repository.ProviderModelUpdateRepo, svc *service.DeepInfraCatalogService) *DeepInfraModelsHandler {
	return &DeepInfraModelsHandler{
		repo:               repo,
		settingsRepo:       settingsRepo,
		service:            svc,
		cipher:             cipher,
		providerUpdateRepo: providerUpdateRepo,
	}
}

func (h *DeepInfraModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	models, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if models == nil {
		models = make([]repository.DeepInfraModelSnapshot, 0)
	}
	prevModels := []repository.DeepInfraModelSnapshot{}
	if latestRun != nil {
		prevModels, err = h.repo.ListPreviousSuccessfulSnapshots(r.Context(), latestRun.ID)
		if err != nil {
			writeRepoError(w, err)
			return
		}
	}
	recentChanges := map[string]string(nil)
	if latestRun != nil && latestRun.TriggerType == "manual" {
		recentChanges = buildDeepInfraRecentChanges(prevModels, models)
	}
	availableModels, unavailableModels := splitDeepInfraModelEntries(models, prevModels, recentChanges)
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "deepinfra")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"latest_change_summary": latestChangeSummary,
		"models":                availableModels,
		"unavailable_models":    unavailableModels,
	})
}

func (h *DeepInfraModelsHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestManualRunningSyncRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"run": run})
}

func (h *DeepInfraModelsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	apiKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetDeepInfraAPIKeyEncrypted, h.cipher, userID, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
		http.Error(w, "deepinfra api key is not configured", http.StatusBadRequest)
		return
	}

	syncRunID, err := h.repo.StartSyncRun(r.Context(), "manual")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	models, fetchErr := h.service.FetchModels(r.Context(), *apiKey)
	if fetchErr != nil {
		msg := fetchErr.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, 0, 0, &msg)
		http.Error(w, fetchErr.Error(), http.StatusBadGateway)
		return
	}
	if cache, err := h.repo.ListLatestDescriptionCache(r.Context()); err == nil {
		models, _ = service.ApplyDeepInfraDescriptionCache(models, cache)
	}
	fetchedAt := time.Now().UTC()
	if err := h.repo.InsertSnapshots(r.Context(), syncRunID, fetchedAt, models); err != nil {
		msg := err.Error()
		_ = h.repo.FinishSyncRun(r.Context(), syncRunID, len(models), 0, &msg)
		writeRepoError(w, err)
		return
	}
	total, completed := deepInfraTranslationProgress(models)
	if err := h.repo.UpdateTranslationProgress(r.Context(), syncRunID, total, completed); err != nil {
		writeRepoError(w, err)
		return
	}

	service.SetDynamicChatModelsForProvider("deepinfra", service.DeepInfraSnapshotsToCatalogModels(models))

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
		if err := h.insertDeepInfraChangeEvents(r.Context(), latestRun.TriggerType, prevModels, latest); err != nil {
			writeRepoError(w, err)
			return
		}
	}
	recentChanges := map[string]string(nil)
	if latestRun != nil && latestRun.TriggerType == "manual" {
		recentChanges = buildDeepInfraRecentChanges(prevModels, latest)
	}
	availableModels, unavailableModels := splitDeepInfraModelEntries(latest, prevModels, recentChanges)
	var summary any
	if h.providerUpdateRepo != nil {
		summary, err = h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "deepinfra")
		if err != nil {
			writeRepoError(w, err)
			return
		}
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"latest_change_summary": summary,
		"models":                availableModels,
		"unavailable_models":    unavailableModels,
	})

	go h.translateDescriptions(syncRunID, models)
}

func buildDeepInfraModelEntry(item repository.DeepInfraModelSnapshot) deepInfraModelsResponseEntry {
	return deepInfraModelsResponseEntry{
		ModelID:             item.ModelID,
		DisplayName:         item.DisplayName,
		ProviderSlug:        item.ProviderSlug,
		ModelType:           item.ReportedType,
		DescriptionEN:       item.DescriptionEN,
		DescriptionJA:       item.DescriptionJA,
		ContextLength:       item.ContextLength,
		MaxTokens:           item.MaxTokens,
		InputPerMTokUSD:     item.InputPerMTokUSD,
		OutputPerMTokUSD:    item.OutputPerMTokUSD,
		CacheReadPerMTokUSD: item.CacheReadPerMTokUSD,
		FetchedAt:           item.FetchedAt,
	}
}

func (h *DeepInfraModelsHandler) translateDescriptions(syncRunID string, models []repository.DeepInfraModelSnapshot) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			msg := "DeepInfra description translation panicked"
			log.Printf("deepinfra description translation panic sync_run_id=%s panic=%v", syncRunID, recovered)
			_ = h.repo.FailSyncRun(context.Background(), syncRunID, msg)
		}
	}()
	openAI := service.NewOpenAIClient()
	total, completed := deepInfraTranslationProgress(models)
	pending := deepInfraPendingTranslationModels(models)
	for _, pendingModel := range pending {
		enriched := service.EnrichDeepInfraDescriptionsJA(ctx, h.repo, openAI, []repository.DeepInfraModelSnapshot{pendingModel})
		if len(enriched) == 0 {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, "empty translation response")
			completed++
			_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
			continue
		}
		ja := strings.TrimSpace(derefOptionalString(enriched[0].DescriptionJA))
		descEN := strings.TrimSpace(derefOptionalString(pendingModel.DescriptionEN))
		if ja == "" || ja == descEN {
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, "translation unavailable")
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
			log.Printf("deepinfra description translation update failed sync_run_id=%s model_id=%s err=%v", syncRunID, pendingModel.ModelID, err)
			_ = h.repo.RecordTranslationFailure(ctx, syncRunID, err.Error())
			msg := err.Error()
			_ = h.repo.FinishSyncRun(ctx, syncRunID, len(models), len(models), &msg)
			return
		}
		completed++
		_ = h.repo.UpdateTranslationProgress(ctx, syncRunID, total, completed)
	}
	service.SetDynamicChatModelsForProvider("deepinfra", service.DeepInfraSnapshotsToCatalogModels(models))
	if err := h.repo.FinishSyncRun(ctx, syncRunID, len(models), len(models), nil); err != nil {
		log.Printf("deepinfra sync finish failed sync_run_id=%s err=%v", syncRunID, err)
	}
}

func (h *DeepInfraModelsHandler) insertDeepInfraChangeEvents(ctx context.Context, trigger string, previous, current []repository.DeepInfraModelSnapshot) error {
	if h.providerUpdateRepo == nil {
		return nil
	}
	events := buildDeepInfraChangeEvents(trigger, time.Now().UTC(), previous, current)
	if len(events) == 0 {
		return nil
	}
	return h.providerUpdateRepo.InsertChangeEvents(ctx, events)
}

func buildDeepInfraChangeEvents(trigger string, detectedAt time.Time, previous, current []repository.DeepInfraModelSnapshot) []model.ProviderModelChangeEvent {
	prevByID := make(map[string]repository.DeepInfraModelSnapshot, len(previous))
	for _, item := range previous {
		prevByID[item.ModelID] = item
	}
	currByID := make(map[string]repository.DeepInfraModelSnapshot, len(current))
	events := make([]model.ProviderModelChangeEvent, 0, len(previous)+len(current))
	for _, item := range current {
		currByID[item.ModelID] = item
		metadata := map[string]any{"source": "deepinfra_sync", "trigger": trigger}
		prev, ok := prevByID[item.ModelID]
		if !ok {
			events = append(events, model.ProviderModelChangeEvent{
				Provider:   "deepinfra",
				ChangeType: "added",
				ModelID:    item.ModelID,
				DetectedAt: detectedAt,
				Metadata:   metadata,
			})
			continue
		}
		if !deepInfraFloatPtrEqual(prev.InputPerMTokUSD, item.InputPerMTokUSD) || !deepInfraFloatPtrEqual(prev.OutputPerMTokUSD, item.OutputPerMTokUSD) {
			events = append(events, model.ProviderModelChangeEvent{
				Provider:   "deepinfra",
				ChangeType: "pricing_changed",
				ModelID:    item.ModelID,
				DetectedAt: detectedAt,
				Metadata:   metadata,
			})
		}
		if !deepInfraIntPtrEqual(prev.ContextLength, item.ContextLength) {
			events = append(events, model.ProviderModelChangeEvent{
				Provider:   "deepinfra",
				ChangeType: "context_changed",
				ModelID:    item.ModelID,
				DetectedAt: detectedAt,
				Metadata:   metadata,
			})
		}
	}
	for _, item := range previous {
		if _, ok := currByID[item.ModelID]; ok {
			continue
		}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "deepinfra",
			ChangeType: "removed",
			ModelID:    item.ModelID,
			DetectedAt: detectedAt,
			Metadata:   map[string]any{"source": "deepinfra_sync", "trigger": trigger},
		})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].ChangeType == events[j].ChangeType {
			return events[i].ModelID < events[j].ModelID
		}
		return events[i].ChangeType < events[j].ChangeType
	})
	return events
}

func deepInfraIntPtrEqual(a, b *int) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

func deepInfraFloatPtrEqual(a, b *float64) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

func deepInfraTranslationProgress(models []repository.DeepInfraModelSnapshot) (total, completed int) {
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

func deepInfraPendingTranslationModels(models []repository.DeepInfraModelSnapshot) []repository.DeepInfraModelSnapshot {
	pending := make([]repository.DeepInfraModelSnapshot, 0)
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

func splitDeepInfraModelEntries(current, previous []repository.DeepInfraModelSnapshot, recentChanges map[string]string) ([]deepInfraModelListEntry, []deepInfraModelListEntry) {
	available := make([]deepInfraModelListEntry, 0, len(current))
	unavailable := make([]deepInfraModelListEntry, 0)
	currentMap := make(map[string]repository.DeepInfraModelSnapshot, len(current))
	for _, item := range current {
		currentMap[item.ModelID] = item
		entry := deepInfraModelListEntry{
			deepInfraModelsResponseEntry: buildDeepInfraModelEntry(item),
			Availability:                 "available",
		}
		if recentChange, ok := recentChanges[item.ModelID]; ok {
			rc := recentChange
			entry.RecentChange = &rc
		}
		available = append(available, entry)
	}
	for _, item := range previous {
		if _, exists := currentMap[item.ModelID]; exists {
			continue
		}
		entry := deepInfraModelListEntry{
			deepInfraModelsResponseEntry: buildDeepInfraModelEntry(item),
			Availability:                 "removed",
		}
		if recentChange, ok := recentChanges[item.ModelID]; ok {
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
		if unavailable[i].ProviderSlug == unavailable[j].ProviderSlug {
			return unavailable[i].DisplayName < unavailable[j].DisplayName
		}
		return unavailable[i].ProviderSlug < unavailable[j].ProviderSlug
	})
	return available, unavailable
}

func buildDeepInfraRecentChanges(previous, current []repository.DeepInfraModelSnapshot) map[string]string {
	prevMap := make(map[string]repository.DeepInfraModelSnapshot, len(previous))
	for _, item := range previous {
		prevMap[item.ModelID] = item
	}
	currMap := make(map[string]repository.DeepInfraModelSnapshot, len(current))
	changes := make(map[string]string)
	for _, item := range current {
		currMap[item.ModelID] = item
		prev, existed := prevMap[item.ModelID]
		if !existed {
			changes[item.ModelID] = "added"
			continue
		}
		if !deepInfraFloatPtrEqual(prev.InputPerMTokUSD, item.InputPerMTokUSD) || !deepInfraFloatPtrEqual(prev.OutputPerMTokUSD, item.OutputPerMTokUSD) {
			changes[item.ModelID] = "pricing_changed"
			continue
		}
		if !deepInfraIntPtrEqual(prev.ContextLength, item.ContextLength) {
			changes[item.ModelID] = "context_changed"
		}
	}
	for _, item := range previous {
		if _, exists := currMap[item.ModelID]; exists {
			continue
		}
		changes[item.ModelID] = "removed"
	}
	return changes
}
