package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testSettingsHandlerDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockSettingsHandlerTestDB(t, pool)

	if _, err := pool.Exec(context.Background(), `
		DELETE FROM user_settings WHERE user_id = '00000000-0000-4000-8000-000000000051';
		DELETE FROM users WHERE id = '00000000-0000-4000-8000-000000000051';
		INSERT INTO users (id, email, name)
		VALUES ('00000000-0000-4000-8000-000000000051', 'settings-handler@example.com', 'Settings Handler');
	`); err != nil {
		t.Fatalf("reset settings handler tables: %v", err)
	}

	return pool
}

func lockSettingsHandlerTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231006
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
}

func newSettingsHandlerForAPIKeyTest(t *testing.T) *SettingsHandler {
	t.Helper()

	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "settings-handler-minimax-test-key")
	pool := testSettingsHandlerDB(t)
	return &SettingsHandler{
		settings: service.NewSettingsService(
			repository.NewUserSettingsRepo(pool),
			repository.NewUserRepo(pool),
			nil,
			nil,
			nil,
			repository.NewObsidianExportRepo(pool),
			nil,
			nil,
			service.NewSecretCipher(),
			nil,
		),
	}
}

func TestSettingsHandlerSetMiniMaxAPIKey(t *testing.T) {
	handler := newSettingsHandlerForAPIKeyTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/minimax-key", bytes.NewBufferString(`{"api_key":"minimax-handler-key"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "00000000-0000-4000-8000-000000000051"))
	rec := httptest.NewRecorder()

	handler.SetMiniMaxAPIKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UserID             string  `json:"user_id"`
		HasMiniMaxAPIKey   bool    `json:"has_minimax_api_key"`
		MiniMaxAPIKeyLast4 *string `json:"minimax_api_key_last4"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.HasMiniMaxAPIKey {
		t.Fatal("has_minimax_api_key = false, want true")
	}
	if resp.MiniMaxAPIKeyLast4 == nil || *resp.MiniMaxAPIKeyLast4 != "-key" {
		t.Fatalf("minimax_api_key_last4 = %#v, want %q", resp.MiniMaxAPIKeyLast4, "-key")
	}
}

func TestSettingsHandlerDeleteMiniMaxAPIKey(t *testing.T) {
	handler := newSettingsHandlerForAPIKeyTest(t)
	userID := "00000000-0000-4000-8000-000000000051"

	setReq := httptest.NewRequest(http.MethodPost, "/api/settings/minimax-key", bytes.NewBufferString(`{"api_key":"minimax-handler-key"}`))
	setReq = setReq.WithContext(context.WithValue(setReq.Context(), middleware.UserIDKey, userID))
	setRec := httptest.NewRecorder()
	handler.SetMiniMaxAPIKey(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want 200 body=%s", setRec.Code, setRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/minimax-key", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.DeleteMiniMaxAPIKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		HasMiniMaxAPIKey   bool    `json:"has_minimax_api_key"`
		MiniMaxAPIKeyLast4 *string `json:"minimax_api_key_last4"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.HasMiniMaxAPIKey {
		t.Fatal("has_minimax_api_key = true, want false")
	}
	if resp.MiniMaxAPIKeyLast4 != nil {
		t.Fatalf("minimax_api_key_last4 = %#v, want nil", resp.MiniMaxAPIKeyLast4)
	}
}

func TestSettingsHandlerSetXiaomiMiMoTokenPlanAPIKey(t *testing.T) {
	handler := newSettingsHandlerForAPIKeyTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/xiaomi-mimo-token-plan-key", bytes.NewBufferString(`{"api_key":"mimo-handler-key"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "00000000-0000-4000-8000-000000000051"))
	rec := httptest.NewRecorder()

	handler.SetXiaomiMiMoTokenPlanAPIKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UserID                         string  `json:"user_id"`
		HasXiaomiMiMoTokenPlanAPIKey   bool    `json:"has_xiaomi_mimo_token_plan_api_key"`
		XiaomiMiMoTokenPlanAPIKeyLast4 *string `json:"xiaomi_mimo_token_plan_api_key_last4"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.HasXiaomiMiMoTokenPlanAPIKey {
		t.Fatal("has_xiaomi_mimo_token_plan_api_key = false, want true")
	}
	if resp.XiaomiMiMoTokenPlanAPIKeyLast4 == nil || *resp.XiaomiMiMoTokenPlanAPIKeyLast4 != "-key" {
		t.Fatalf("xiaomi_mimo_token_plan_api_key_last4 = %#v, want %q", resp.XiaomiMiMoTokenPlanAPIKeyLast4, "-key")
	}
}

func TestSettingsHandlerDeleteXiaomiMiMoTokenPlanAPIKey(t *testing.T) {
	handler := newSettingsHandlerForAPIKeyTest(t)
	userID := "00000000-0000-4000-8000-000000000051"

	setReq := httptest.NewRequest(http.MethodPost, "/api/settings/xiaomi-mimo-token-plan-key", bytes.NewBufferString(`{"api_key":"mimo-handler-key"}`))
	setReq = setReq.WithContext(context.WithValue(setReq.Context(), middleware.UserIDKey, userID))
	setRec := httptest.NewRecorder()
	handler.SetXiaomiMiMoTokenPlanAPIKey(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want 200 body=%s", setRec.Code, setRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/xiaomi-mimo-token-plan-key", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.DeleteXiaomiMiMoTokenPlanAPIKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		HasXiaomiMiMoTokenPlanAPIKey   bool    `json:"has_xiaomi_mimo_token_plan_api_key"`
		XiaomiMiMoTokenPlanAPIKeyLast4 *string `json:"xiaomi_mimo_token_plan_api_key_last4"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.HasXiaomiMiMoTokenPlanAPIKey {
		t.Fatal("has_xiaomi_mimo_token_plan_api_key = true, want false")
	}
	if resp.XiaomiMiMoTokenPlanAPIKeyLast4 != nil {
		t.Fatalf("xiaomi_mimo_token_plan_api_key_last4 = %#v, want nil", resp.XiaomiMiMoTokenPlanAPIKeyLast4)
	}
}

func TestSettingsHandlerSetFeatherlessAPIKey(t *testing.T) {
	handler := newSettingsHandlerForAPIKeyTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/featherless-key", bytes.NewBufferString(`{"api_key":"featherless-handler-key"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "00000000-0000-4000-8000-000000000051"))
	rec := httptest.NewRecorder()

	handler.SetFeatherlessAPIKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UserID                 string  `json:"user_id"`
		HasFeatherlessAPIKey   bool    `json:"has_featherless_api_key"`
		FeatherlessAPIKeyLast4 *string `json:"featherless_api_key_last4"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.HasFeatherlessAPIKey {
		t.Fatal("has_featherless_api_key = false, want true")
	}
	if resp.FeatherlessAPIKeyLast4 == nil || *resp.FeatherlessAPIKeyLast4 != "-key" {
		t.Fatalf("featherless_api_key_last4 = %#v, want %q", resp.FeatherlessAPIKeyLast4, "-key")
	}
}

func TestSettingsHandlerDeleteFeatherlessAPIKey(t *testing.T) {
	handler := newSettingsHandlerForAPIKeyTest(t)
	userID := "00000000-0000-4000-8000-000000000051"

	setReq := httptest.NewRequest(http.MethodPost, "/api/settings/featherless-key", bytes.NewBufferString(`{"api_key":"featherless-handler-key"}`))
	setReq = setReq.WithContext(context.WithValue(setReq.Context(), middleware.UserIDKey, userID))
	setRec := httptest.NewRecorder()
	handler.SetFeatherlessAPIKey(setRec, setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want 200 body=%s", setRec.Code, setRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/featherless-key", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.DeleteFeatherlessAPIKey(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		HasFeatherlessAPIKey   bool    `json:"has_featherless_api_key"`
		FeatherlessAPIKeyLast4 *string `json:"featherless_api_key_last4"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.HasFeatherlessAPIKey {
		t.Fatal("has_featherless_api_key = true, want false")
	}
	if resp.FeatherlessAPIKeyLast4 != nil {
		t.Fatalf("featherless_api_key_last4 = %#v, want nil", resp.FeatherlessAPIKeyLast4)
	}
}
