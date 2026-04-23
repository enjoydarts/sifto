package repository

import (
	"context"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testProviderModelUpdatesRepoDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockProviderModelUpdatesRepoTestDB(t, pool)

	if _, err := pool.Exec(context.Background(), `
		DELETE FROM provider_model_change_events WHERE provider = 'minimax';
		DELETE FROM provider_model_snapshots WHERE provider = 'minimax';
		DELETE FROM provider_model_change_events WHERE provider = 'featherless';
		DELETE FROM provider_model_snapshots WHERE provider = 'featherless';
		DELETE FROM provider_model_change_events WHERE provider = 'deepinfra';
		DELETE FROM provider_model_snapshots WHERE provider = 'deepinfra';
	`); err != nil {
		t.Fatalf("reset provider model updates tables: %v", err)
	}

	return pool
}

func lockProviderModelUpdatesRepoTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231047
	if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
	if _, err := pool.Exec(context.Background(), `
		ALTER TABLE provider_model_change_events
		  DROP CONSTRAINT IF EXISTS provider_model_change_events_change_type_check
	`); err != nil {
		t.Fatalf("drop provider_model_change_events_change_type_check: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		ALTER TABLE provider_model_change_events
		  ADD CONSTRAINT provider_model_change_events_change_type_check
		  CHECK (change_type IN ('added', 'constrained', 'availability_changed', 'gated_changed', 'pricing_changed', 'context_changed', 'removed'))
	`); err != nil {
		t.Fatalf("add provider_model_change_events_change_type_check: %v", err)
	}
}

func TestProviderModelUpdateRepoListSnapshotEntriesIncludesFailedProviderWithoutModels(t *testing.T) {
	ctx := context.Background()
	pool := testProviderModelUpdatesRepoDB(t)
	repo := NewProviderModelUpdateRepo(pool)
	errText := "status 500 body=upstream failed"

	if err := repo.UpsertSnapshot(ctx, "minimax", nil, "failed", &errText); err != nil {
		t.Fatalf("UpsertSnapshot() error = %v", err)
	}

	items, total, err := repo.ListSnapshotEntries(ctx, []string{"minimax"}, "", 100, 0)
	if err != nil {
		t.Fatalf("ListSnapshotEntries() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Provider != "minimax" {
		t.Fatalf("provider = %q, want %q", items[0].Provider, "minimax")
	}
	if items[0].ModelID != "" {
		t.Fatalf("model_id = %q, want empty", items[0].ModelID)
	}
	if items[0].Status != "failed" {
		t.Fatalf("status = %q, want %q", items[0].Status, "failed")
	}
	if items[0].Error == nil || *items[0].Error != errText {
		t.Fatalf("error = %#v, want %q", items[0].Error, errText)
	}
}

func TestProviderModelUpdateRepoListLatestProviderSummaryIncludesAvailabilityAndGatedChanges(t *testing.T) {
	ctx := context.Background()
	pool := testProviderModelUpdatesRepoDB(t)
	repo := NewProviderModelUpdateRepo(pool)
	detectedAt := time.Date(2026, 4, 19, 3, 4, 5, 0, time.UTC)

	events := []model.ProviderModelChangeEvent{
		{Provider: "featherless", ChangeType: "added", ModelID: "added-model", DetectedAt: detectedAt, Metadata: map[string]any{"trigger": "manual"}},
		{Provider: "featherless", ChangeType: "availability_changed", ModelID: "availability-model", DetectedAt: detectedAt, Metadata: map[string]any{"trigger": "manual"}},
		{Provider: "featherless", ChangeType: "gated_changed", ModelID: "gated-model", DetectedAt: detectedAt, Metadata: map[string]any{"trigger": "manual"}},
		{Provider: "featherless", ChangeType: "removed", ModelID: "removed-model", DetectedAt: detectedAt, Metadata: map[string]any{"trigger": "manual"}},
	}
	if err := repo.InsertChangeEvents(ctx, events); err != nil {
		t.Fatalf("InsertChangeEvents() error = %v", err)
	}

	summary, err := repo.ListLatestProviderSummary(ctx, "featherless")
	if err != nil {
		t.Fatalf("ListLatestProviderSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("summary = nil, want summary")
	}
	if len(summary.Added) != 1 || summary.Added[0].ModelID != "added-model" {
		t.Fatalf("added = %#v, want added-model", summary.Added)
	}
	if len(summary.AvailabilityChanged) != 1 || summary.AvailabilityChanged[0].ModelID != "availability-model" {
		t.Fatalf("availability_changed = %#v, want availability-model", summary.AvailabilityChanged)
	}
	if len(summary.GatedChanged) != 1 || summary.GatedChanged[0].ModelID != "gated-model" {
		t.Fatalf("gated_changed = %#v, want gated-model", summary.GatedChanged)
	}
	if len(summary.Removed) != 1 || summary.Removed[0].ModelID != "removed-model" {
		t.Fatalf("removed = %#v, want removed-model", summary.Removed)
	}
}

func TestProviderModelUpdateRepoListLatestProviderSummaryIncludesPricingAndContextChanges(t *testing.T) {
	ctx := context.Background()
	pool := testProviderModelUpdatesRepoDB(t)
	repo := NewProviderModelUpdateRepo(pool)
	detectedAt := time.Date(2026, 4, 23, 3, 4, 5, 0, time.UTC)

	events := []model.ProviderModelChangeEvent{
		{Provider: "deepinfra", ChangeType: "pricing_changed", ModelID: "pricing-model", DetectedAt: detectedAt, Metadata: map[string]any{"trigger": "manual"}},
		{Provider: "deepinfra", ChangeType: "context_changed", ModelID: "context-model", DetectedAt: detectedAt, Metadata: map[string]any{"trigger": "manual"}},
	}
	if err := repo.InsertChangeEvents(ctx, events); err != nil {
		t.Fatalf("InsertChangeEvents() error = %v", err)
	}

	summary, err := repo.ListLatestProviderSummary(ctx, "deepinfra")
	if err != nil {
		t.Fatalf("ListLatestProviderSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("summary = nil, want summary")
	}
	if len(summary.PricingChanged) != 1 || summary.PricingChanged[0].ModelID != "pricing-model" {
		t.Fatalf("pricing_changed = %#v, want pricing-model", summary.PricingChanged)
	}
	if len(summary.ContextChanged) != 1 || summary.ContextChanged[0].ModelID != "context-model" {
		t.Fatalf("context_changed = %#v, want context-model", summary.ContextChanged)
	}
}
