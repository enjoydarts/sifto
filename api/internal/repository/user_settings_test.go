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
	if _, err := pool.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS featherless_api_key_enc text`); err != nil {
		t.Fatalf("ensure user_settings.featherless_api_key_enc: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS featherless_api_key_last4 text`); err != nil {
		t.Fatalf("ensure user_settings.featherless_api_key_last4: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS deepinfra_api_key_enc text`); err != nil {
		t.Fatalf("ensure user_settings.deepinfra_api_key_enc: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS deepinfra_api_key_last4 text`); err != nil {
		t.Fatalf("ensure user_settings.deepinfra_api_key_last4: %v", err)
	}
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

func TestUserSettingsRepoSetAndClearXiaomiMiMoTokenPlanAPIKey(t *testing.T) {
	ctx := context.Background()
	pool := testUserSettingsRepoDB(t)
	repo := NewUserSettingsRepo(pool)
	userID := "00000000-0000-4000-8000-000000000041"

	settings, err := repo.SetXiaomiMiMoTokenPlanAPIKey(ctx, userID, "encrypted-mimo-key", "5678")
	if err != nil {
		t.Fatalf("SetXiaomiMiMoTokenPlanAPIKey() error = %v", err)
	}
	if !settings.HasXiaomiMiMoTokenPlanAPIKey {
		t.Fatal("HasXiaomiMiMoTokenPlanAPIKey = false, want true")
	}
	if settings.XiaomiMiMoTokenPlanAPIKeyLast4 == nil || *settings.XiaomiMiMoTokenPlanAPIKeyLast4 != "5678" {
		t.Fatalf("XiaomiMiMoTokenPlanAPIKeyLast4 = %#v, want %q", settings.XiaomiMiMoTokenPlanAPIKeyLast4, "5678")
	}

	enc, err := repo.GetXiaomiMiMoTokenPlanAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetXiaomiMiMoTokenPlanAPIKeyEncrypted() error = %v", err)
	}
	if enc == nil || *enc != "encrypted-mimo-key" {
		t.Fatalf("encrypted key = %#v, want %q", enc, "encrypted-mimo-key")
	}

	settings, err = repo.ClearXiaomiMiMoTokenPlanAPIKey(ctx, userID)
	if err != nil {
		t.Fatalf("ClearXiaomiMiMoTokenPlanAPIKey() error = %v", err)
	}
	if settings.HasXiaomiMiMoTokenPlanAPIKey {
		t.Fatal("HasXiaomiMiMoTokenPlanAPIKey = true, want false")
	}
	if settings.XiaomiMiMoTokenPlanAPIKeyLast4 != nil {
		t.Fatalf("XiaomiMiMoTokenPlanAPIKeyLast4 = %#v, want nil", settings.XiaomiMiMoTokenPlanAPIKeyLast4)
	}

	enc, err = repo.GetXiaomiMiMoTokenPlanAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetXiaomiMiMoTokenPlanAPIKeyEncrypted() after clear error = %v", err)
	}
	if enc != nil {
		t.Fatalf("encrypted key after clear = %#v, want nil", enc)
	}
}

func TestUserSettingsRepoSetAndClearFeatherlessAPIKey(t *testing.T) {
	ctx := context.Background()
	pool := testUserSettingsRepoDB(t)
	repo := NewUserSettingsRepo(pool)
	userID := "00000000-0000-4000-8000-000000000041"

	settings, err := repo.SetFeatherlessAPIKey(ctx, userID, "encrypted-featherless-key", "9012")
	if err != nil {
		t.Fatalf("SetFeatherlessAPIKey() error = %v", err)
	}
	if !settings.HasFeatherlessAPIKey {
		t.Fatal("HasFeatherlessAPIKey = false, want true")
	}
	if settings.FeatherlessAPIKeyLast4 == nil || *settings.FeatherlessAPIKeyLast4 != "9012" {
		t.Fatalf("FeatherlessAPIKeyLast4 = %#v, want %q", settings.FeatherlessAPIKeyLast4, "9012")
	}

	enc, err := repo.GetFeatherlessAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetFeatherlessAPIKeyEncrypted() error = %v", err)
	}
	if enc == nil || *enc != "encrypted-featherless-key" {
		t.Fatalf("encrypted key = %#v, want %q", enc, "encrypted-featherless-key")
	}

	settings, err = repo.ClearFeatherlessAPIKey(ctx, userID)
	if err != nil {
		t.Fatalf("ClearFeatherlessAPIKey() error = %v", err)
	}
	if settings.HasFeatherlessAPIKey {
		t.Fatal("HasFeatherlessAPIKey = true, want false")
	}
	if settings.FeatherlessAPIKeyLast4 != nil {
		t.Fatalf("FeatherlessAPIKeyLast4 = %#v, want nil", settings.FeatherlessAPIKeyLast4)
	}

	enc, err = repo.GetFeatherlessAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetFeatherlessAPIKeyEncrypted() after clear error = %v", err)
	}
	if enc != nil {
		t.Fatalf("encrypted key after clear = %#v, want nil", enc)
	}
}

func TestUserSettingsRepoSetAndClearDeepInfraAPIKey(t *testing.T) {
	ctx := context.Background()
	pool := testUserSettingsRepoDB(t)
	repo := NewUserSettingsRepo(pool)
	userID := "00000000-0000-4000-8000-000000000041"

	settings, err := repo.SetDeepInfraAPIKey(ctx, userID, "encrypted-deepinfra-key", "3456")
	if err != nil {
		t.Fatalf("SetDeepInfraAPIKey() error = %v", err)
	}
	if !settings.HasDeepInfraAPIKey {
		t.Fatal("HasDeepInfraAPIKey = false, want true")
	}
	if settings.DeepInfraAPIKeyLast4 == nil || *settings.DeepInfraAPIKeyLast4 != "3456" {
		t.Fatalf("DeepInfraAPIKeyLast4 = %#v, want %q", settings.DeepInfraAPIKeyLast4, "3456")
	}

	enc, err := repo.GetDeepInfraAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetDeepInfraAPIKeyEncrypted() error = %v", err)
	}
	if enc == nil || *enc != "encrypted-deepinfra-key" {
		t.Fatalf("encrypted key = %#v, want %q", enc, "encrypted-deepinfra-key")
	}

	settings, err = repo.ClearDeepInfraAPIKey(ctx, userID)
	if err != nil {
		t.Fatalf("ClearDeepInfraAPIKey() error = %v", err)
	}
	if settings.HasDeepInfraAPIKey {
		t.Fatal("HasDeepInfraAPIKey = true, want false")
	}
	if settings.DeepInfraAPIKeyLast4 != nil {
		t.Fatalf("DeepInfraAPIKeyLast4 = %#v, want nil", settings.DeepInfraAPIKeyLast4)
	}

	enc, err = repo.GetDeepInfraAPIKeyEncrypted(ctx, userID)
	if err != nil {
		t.Fatalf("GetDeepInfraAPIKeyEncrypted() after clear error = %v", err)
	}
	if enc != nil {
		t.Fatalf("encrypted key after clear = %#v, want nil", enc)
	}
}
