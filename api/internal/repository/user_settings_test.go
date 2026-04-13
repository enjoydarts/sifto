package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testUserSettingsRepoDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockUserSettingsRepoTestDB(t, pool)

	if _, err := pool.Exec(context.Background(), `
		DELETE FROM user_settings WHERE user_id = '00000000-0000-4000-8000-000000000041';
		DELETE FROM users WHERE id = '00000000-0000-4000-8000-000000000041';
		INSERT INTO users (id, email, name)
		VALUES ('00000000-0000-4000-8000-000000000041', 'user-settings-repo@example.com', 'User Settings Repo');
	`); err != nil {
		t.Fatalf("reset user settings repo tables: %v", err)
	}

	return pool
}

func lockUserSettingsRepoTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231005
	if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}

func TestUserSettingsRepoSetAndClearMiniMaxAPIKey(t *testing.T) {
	ctx := context.Background()
	pool := testUserSettingsRepoDB(t)
	repo := NewUserSettingsRepo(pool)
	userID := "00000000-0000-4000-8000-000000000041"

	settings, err := repo.SetMiniMaxAPIKey(ctx, userID, "encrypted-minimax-key", "1234")
	if err != nil {
		t.Fatalf("SetMiniMaxAPIKey() error = %v", err)
	}
	if !settings.HasMiniMaxAPIKey {
		t.Fatal("HasMiniMaxAPIKey = false, want true")
	}
	if settings.MiniMaxAPIKeyLast4 == nil || *settings.MiniMaxAPIKeyLast4 != "1234" {
		t.Fatalf("MiniMaxAPIKeyLast4 = %#v, want %q", settings.MiniMaxAPIKeyLast4, "1234")
	}

	enc, err := repo.GetMiniMaxAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetMiniMaxAPIKeyEncrypted() error = %v", err)
	}
	if enc == nil || *enc != "encrypted-minimax-key" {
		t.Fatalf("encrypted key = %#v, want %q", enc, "encrypted-minimax-key")
	}

	settings, err = repo.ClearMiniMaxAPIKey(ctx, userID)
	if err != nil {
		t.Fatalf("ClearMiniMaxAPIKey() error = %v", err)
	}
	if settings.HasMiniMaxAPIKey {
		t.Fatal("HasMiniMaxAPIKey = true, want false")
	}
	if settings.MiniMaxAPIKeyLast4 != nil {
		t.Fatalf("MiniMaxAPIKeyLast4 = %#v, want nil", settings.MiniMaxAPIKeyLast4)
	}

	enc, err = repo.GetMiniMaxAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetMiniMaxAPIKeyEncrypted() after clear error = %v", err)
	}
	if enc != nil {
		t.Fatalf("encrypted key after clear = %#v, want nil", enc)
	}
}
