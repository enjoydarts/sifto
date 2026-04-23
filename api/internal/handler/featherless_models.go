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

type FeatherlessModelsHandler struct {
	repo               *repository.FeatherlessModelRepo
	settingsRepo       *repository.UserSettingsRepo
	service            *service.FeatherlessCatalogService
	cipher             *service.SecretCipher
	providerUpdateRepo *repository.ProviderModelUpdateRepo
}

func NewFeatherlessModelsHandler(repo *repository.FeatherlessModelRepo, settingsRepo *repository.UserSettingsRepo, cipher *service.SecretCipher, providerUpdateRepo *repository.ProviderModelUpdateRepo, svc *service.FeatherlessCatalogService) *FeatherlessModelsHandler {
	return &FeatherlessModelsHandler{
		repo:               repo,
		settingsRepo:       settingsRepo,
		cipher:             cipher,
		providerUpdateRepo: providerUpdateRepo,
		service:            svc,
	}
}

type featherlessModelListEntry struct {
	repository.FeatherlessModelSnapshot
	ProviderSlug    string  `json:"provider_slug"`
	Availability    string  `json:"availability"`
	RawAvailability string  `json:"raw_availability,omitempty"`
	Reason          *string `json:"reason,omitempty"`
	RecentChange    *string `json:"recent_change,omitempty"`
	Gated           bool    `json:"gated"`
}

func (h *FeatherlessModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	models, latestRun, err := h.repo.ListLatestSnapshots(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if models == nil {
		models = make([]repository.FeatherlessModelSnapshot, 0)
	}
	var latestChangeSummary any
	if h.providerUpdateRepo != nil {
		summary, err := h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "featherless")
		if err != nil {
			writeRepoError(w, err)
			return
		}
		latestChangeSummary = summary
	}
	recentChanges := buildFeatherlessRecentChanges(nil, models)
	availableModels, unavailableModels := splitFeatherlessModelEntries(models, nil, recentChanges)
	if latestRun != nil && h.providerUpdateRepo != nil {
		prevModels, err := h.repo.ListPreviousSuccessfulSnapshots(r.Context(), latestRun.ID)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		recentChanges = buildFeatherlessRecentChanges(prevModels, models)
		availableModels, unavailableModels = splitFeatherlessModelEntries(models, prevModels, recentChanges)
	}
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                availableModels,
		"unavailable_models":    unavailableModels,
		"latest_change_summary": latestChangeSummary,
	})
}

func (h *FeatherlessModelsHandler) Status(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLatestManualRunningSyncRun(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"run": run})
}

func (h *FeatherlessModelsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	apiKey, err := loadAndDecryptUserSecret(r.Context(), h.settingsRepo.GetFeatherlessAPIKeyEncrypted, h.cipher, userID, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
		http.Error(w, "featherless api key is not configured", http.StatusBadRequest)
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
	service.SetDynamicChatModelsForProvider("featherless", service.FeatherlessSnapshotsToCatalogModels(models))

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
		if err := h.insertFeatherlessChangeEvents(r.Context(), latestRun.TriggerType, prevModels, latest); err != nil {
			writeRepoError(w, err)
			return
		}
	}
	var summary any
	if h.providerUpdateRepo != nil {
		summary, err = h.providerUpdateRepo.ListLatestProviderSummary(r.Context(), "featherless")
		if err != nil {
			writeRepoError(w, err)
			return
		}
	}
	recentChanges := buildFeatherlessRecentChanges(prevModels, latest)
	availableModels, unavailableModels := splitFeatherlessModelEntries(latest, prevModels, recentChanges)
	writeJSON(w, map[string]any{
		"latest_run":            latestRun,
		"models":                availableModels,
		"unavailable_models":    unavailableModels,
		"latest_change_summary": summary,
	})
}

func buildFeatherlessRecentChanges(previous, current []repository.FeatherlessModelSnapshot) map[string]string {
	prevByID := make(map[string]repository.FeatherlessModelSnapshot, len(previous))
	for _, item := range previous {
		prevByID[item.ModelID] = item
	}
	currByID := make(map[string]repository.FeatherlessModelSnapshot, len(current))
	out := make(map[string]string)
	for _, item := range current {
		currByID[item.ModelID] = item
		if prev, ok := prevByID[item.ModelID]; !ok {
			out[item.ModelID] = "added"
		} else if prev.AvailableOnCurrentPlan != item.AvailableOnCurrentPlan {
			out[item.ModelID] = "availability_changed"
		} else if prev.IsGated != item.IsGated {
			out[item.ModelID] = "gated_changed"
		}
	}
	for _, item := range previous {
		if _, ok := currByID[item.ModelID]; !ok {
			out[item.ModelID] = "removed"
		}
	}
	return out
}

func splitFeatherlessModelEntries(current, previous []repository.FeatherlessModelSnapshot, recentChanges map[string]string) ([]featherlessModelListEntry, []featherlessModelListEntry) {
	prevByID := make(map[string]repository.FeatherlessModelSnapshot, len(previous))
	for _, item := range previous {
		prevByID[item.ModelID] = item
	}
	available := make([]featherlessModelListEntry, 0, len(current))
	unavailable := make([]featherlessModelListEntry, 0, len(current)+len(previous))
	currentIDs := make(map[string]struct{}, len(current))
	for _, item := range current {
		currentIDs[item.ModelID] = struct{}{}
		entry := featherlessModelListEntry{
			FeatherlessModelSnapshot: item,
			ProviderSlug:             featherlessSourceProvider(item.ModelID),
			Availability:             "available",
			RawAvailability:          "available",
			Gated:                    item.IsGated,
		}
		if change := strings.TrimSpace(recentChanges[item.ModelID]); change != "" {
			entry.RecentChange = &change
		}
		if !item.AvailableOnCurrentPlan {
			entry.Availability = "unavailable"
			entry.RawAvailability = "not_on_plan"
			reason := "not_on_plan"
			entry.Reason = &reason
			unavailable = append(unavailable, entry)
			continue
		}
		available = append(available, entry)
	}
	for _, item := range previous {
		if _, ok := currentIDs[item.ModelID]; ok {
			continue
		}
		entry := featherlessModelListEntry{
			FeatherlessModelSnapshot: item,
			ProviderSlug:             featherlessSourceProvider(item.ModelID),
			Availability:             "removed",
			RawAvailability:          "removed",
			Gated:                    item.IsGated,
		}
		reason := "removed"
		entry.Reason = &reason
		change := "removed"
		entry.RecentChange = &change
		unavailable = append(unavailable, entry)
	}
	sort.Slice(available, func(i, j int) bool {
		if available[i].Gated == available[j].Gated {
			return available[i].DisplayName < available[j].DisplayName
		}
		return !available[i].Gated && available[j].Gated
	})
	sort.Slice(unavailable, func(i, j int) bool {
		unavailableRank := func(v string) int {
			switch v {
			case "unavailable":
				return 0
			case "removed":
				return 1
			default:
				return 2
			}
		}
		if unavailableRank(unavailable[i].Availability) == unavailableRank(unavailable[j].Availability) {
			return unavailable[i].DisplayName < unavailable[j].DisplayName
		}
		return unavailableRank(unavailable[i].Availability) < unavailableRank(unavailable[j].Availability)
	})
	return available, unavailable
}

func (h *FeatherlessModelsHandler) insertFeatherlessChangeEvents(ctx context.Context, trigger string, previous, current []repository.FeatherlessModelSnapshot) error {
	if h.providerUpdateRepo == nil {
		return nil
	}
	now := time.Now().UTC()
	changes := buildFeatherlessRecentChanges(previous, current)
	events := make([]model.ProviderModelChangeEvent, 0, len(changes))
	for modelID, change := range changes {
		metadata := map[string]any{"source": "featherless_sync", "trigger": trigger}
		events = append(events, model.ProviderModelChangeEvent{
			Provider:   "featherless",
			ChangeType: change,
			ModelID:    modelID,
			DetectedAt: now,
			Metadata:   metadata,
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

func featherlessSourceProvider(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	parts := strings.SplitN(modelID, "/", 2)
	return strings.TrimSpace(parts[0])
}
