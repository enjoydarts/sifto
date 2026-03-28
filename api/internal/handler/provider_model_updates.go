package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type providerModelUpdateStore interface {
	ListRecent(ctx context.Context, since time.Time, limit int) ([]model.ProviderModelChangeEvent, error)
	ListSnapshotEntries(ctx context.Context, providers []string, query string, limit, offset int) ([]model.ProviderModelSnapshotEntry, int, error)
	ListSnapshotProviders(ctx context.Context) ([]string, error)
}

type providerModelSnapshotSyncer interface {
	SyncCommonProviders(ctx context.Context, trigger string) (*service.ProviderModelSnapshotSyncSummary, error)
}

type ProviderModelUpdateHandler struct {
	repo   providerModelUpdateStore
	syncer providerModelSnapshotSyncer
}

func NewProviderModelUpdateHandler(repo *repository.ProviderModelUpdateRepo, syncer providerModelSnapshotSyncer) *ProviderModelUpdateHandler {
	return &ProviderModelUpdateHandler{repo: repo, syncer: syncer}
}

func (h *ProviderModelUpdateHandler) ListRecent(w http.ResponseWriter, r *http.Request) {
	days := parseIntOrDefault(r.URL.Query().Get("days"), 14)
	if days < 1 || days > 90 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 30)
	if limit < 1 || limit > 200 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	events, err := h.repo.ListRecent(r.Context(), since, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, events)
}

func (h *ProviderModelUpdateHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 100)
	if limit < 1 || limit > 500 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		http.Error(w, "invalid offset", http.StatusBadRequest)
		return
	}

	providers := make([]string, 0)
	for _, provider := range r.URL.Query()["provider"] {
		for _, part := range strings.Split(provider, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			providers = append(providers, part)
		}
	}

	items, total, err := h.repo.ListSnapshotEntries(r.Context(), providers, r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	allProviders, err := h.repo.ListSnapshotProviders(r.Context())
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, model.ProviderModelSnapshotList{
		Items:     items,
		Providers: allProviders,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	})
}

func (h *ProviderModelUpdateHandler) SyncSnapshots(w http.ResponseWriter, r *http.Request) {
	if h.syncer == nil {
		http.Error(w, "syncer is not configured", http.StatusInternalServerError)
		return
	}
	result, err := h.syncer.SyncCommonProviders(r.Context(), "manual")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, result)
}
