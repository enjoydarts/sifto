package repository

import (
	"context"
	"testing"

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
