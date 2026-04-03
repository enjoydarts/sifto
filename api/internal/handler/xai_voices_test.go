package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

type fakeXAIVoiceFetcher struct {
	voices []repository.XAIVoiceSnapshot
	err    error
}

func (f *fakeXAIVoiceFetcher) FetchVoices(_ context.Context, _ string) ([]repository.XAIVoiceSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]repository.XAIVoiceSnapshot{}, f.voices...), nil
}

type fakeXAISettingsRepo struct {
	encryptedKey *string
	err          error
}

func (f *fakeXAISettingsRepo) GetXAIAPIKeyEncrypted(_ context.Context, _ string) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.encryptedKey, nil
}

func TestXAIVoicesHandlerSync(t *testing.T) {
	const userID = "00000000-0000-4000-8000-000000000001"

	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs, provider_model_change_events, provider_model_snapshots, user_settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, userID, "xai-sync@example.com", "xAI Sync"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "test-secret")
	cipher := service.NewSecretCipher()
	encryptedKey, err := cipher.EncryptString("xai-test-key")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	settingsRepo := repository.NewUserSettingsRepo(db)
	if _, err := settingsRepo.SetXAIAPIKey(context.Background(), userID, encryptedKey, "key"); err != nil {
		t.Fatalf("SetXAIAPIKey() error = %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer xai-test-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"voices": []map[string]any{
				{
					"voice_id":    "voice-1",
					"name":        "Calm",
					"description": "Warm",
					"language":    "en",
					"preview_url": "https://example.com/voice-1.mp3",
					"gender":      "neutral",
				},
			},
		})
	}))
	defer srv.Close()

	voiceRepo := repository.NewXAIVoiceRepo(db)
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	handler := NewXAIVoicesHandler(
		voiceRepo,
		settingsRepo,
		updateRepo,
		cipher,
		service.NewXAIVoiceCatalogServiceWithBaseURL(srv.URL),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/xai-voices/sync", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		LatestRun *repository.XAIVoiceSyncRun   `json:"latest_run"`
		Voices    []repository.XAIVoiceSnapshot `json:"voices"`
		Summary   map[string]any                `json:"latest_change_summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LatestRun == nil || resp.LatestRun.Status != "success" {
		t.Fatalf("latest_run = %#v, want success", resp.LatestRun)
	}
	if len(resp.Voices) != 1 || resp.Voices[0].VoiceID != "voice-1" {
		t.Fatalf("voices = %#v, want voice-1", resp.Voices)
	}

	snapshot, err := updateRepo.GetSnapshot(context.Background(), "xai")
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	if snapshot == nil || len(snapshot.Models) != 1 || snapshot.Models[0] != "voice-1" {
		t.Fatalf("snapshot = %#v, want xai voice-1", snapshot)
	}
}

func TestXAIVoicesHandlerSyncRequiresAPIKey(t *testing.T) {
	const userID = "00000000-0000-4000-8000-000000000002"

	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs, provider_model_change_events, provider_model_snapshots, user_settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, userID, "xai-no-key@example.com", "xAI No Key"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "test-secret")
	cipher := service.NewSecretCipher()
	handler := NewXAIVoicesHandler(
		repository.NewXAIVoiceRepo(db),
		repository.NewUserSettingsRepo(db),
		repository.NewProviderModelUpdateRepo(db),
		cipher,
		service.NewXAIVoiceCatalogServiceWithBaseURL("https://example.com"),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/xai-voices/sync", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "xai api key is not configured") {
		t.Fatalf("body = %q, want xai api key is not configured", rec.Body.String())
	}
}

func TestXAIVoicesHandlerSyncFailedFetchPreservesExistingXAIBaseline(t *testing.T) {
	const userID = "00000000-0000-4000-8000-000000000003"

	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs, provider_model_change_events, provider_model_snapshots, user_settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, userID, "xai-fetch-fail@example.com", "xAI Fetch Fail"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	t.Setenv("USER_SECRET_ENCRYPTION_KEY", "test-secret")
	cipher := service.NewSecretCipher()
	encryptedKey, err := cipher.EncryptString("xai-test-key")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	settingsRepo := repository.NewUserSettingsRepo(db)
	if _, err := settingsRepo.SetXAIAPIKey(context.Background(), userID, encryptedKey, "key"); err != nil {
		t.Fatalf("SetXAIAPIKey() error = %v", err)
	}

	updateRepo := repository.NewProviderModelUpdateRepo(db)
	if err := updateRepo.UpsertSnapshot(context.Background(), "xai", []string{"voice-existing"}, "ok", nil); err != nil {
		t.Fatalf("UpsertSnapshot() error = %v", err)
	}

	handler := NewXAIVoicesHandler(
		repository.NewXAIVoiceRepo(db),
		settingsRepo,
		updateRepo,
		cipher,
		&fakeXAIVoiceFetcher{err: errors.New("upstream failed")},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/xai-voices/sync", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 body=%s", rec.Code, rec.Body.String())
	}

	snapshot, err := updateRepo.GetSnapshot(context.Background(), "xai")
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	if snapshot == nil || len(snapshot.Models) != 1 || snapshot.Models[0] != "voice-existing" {
		t.Fatalf("snapshot = %#v, want preserved voice-existing baseline", snapshot)
	}
	if snapshot.Status != "ok" {
		t.Fatalf("snapshot.Status = %q, want ok", snapshot.Status)
	}
}

func TestXAIVoicesHandlerSyncSecretLoadInternalFailureIsNotReportedAsMissingKey(t *testing.T) {
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs, provider_model_change_events, provider_model_snapshots, user_settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}

	handler := NewXAIVoicesHandler(
		repository.NewXAIVoiceRepo(db),
		&fakeXAISettingsRepo{err: errors.New("repo exploded")},
		nil,
		service.NewSecretCipher(),
		&fakeXAIVoiceFetcher{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/xai-voices/sync", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "00000000-0000-4000-8000-000000000004"))
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "xai api key is not configured") {
		t.Fatalf("body = %q, should not report missing key", rec.Body.String())
	}
}

func TestXAIVoicesHandlerStatusReturnsRunningRun(t *testing.T) {
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs, provider_model_change_events, provider_model_snapshots, user_settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}

	repo := repository.NewXAIVoiceRepo(db)
	runID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}

	handler := &XAIVoicesHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/xai-voices/status", nil)
	rec := httptest.NewRecorder()

	handler.Status(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Run *repository.XAIVoiceSyncRun `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Run == nil || resp.Run.ID != runID || resp.Run.Status != "running" {
		t.Fatalf("run = %#v, want running run %d", resp.Run, runID)
	}
}

func TestXAIVoicesHandlerStatusMarksStaleRunFailed(t *testing.T) {
	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs, provider_model_change_events, provider_model_snapshots, user_settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}

	repo := repository.NewXAIVoiceRepo(db)
	runID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	oldTime := time.Now().UTC().Add(-16 * time.Minute)
	if _, err := db.Exec(context.Background(), `UPDATE xai_voice_sync_runs SET started_at = $2 WHERE id = $1`, runID, oldTime); err != nil {
		t.Fatalf("update run time: %v", err)
	}

	handler := &XAIVoicesHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/xai-voices/status", nil)
	rec := httptest.NewRecorder()

	handler.Status(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Run *repository.XAIVoiceSyncRun `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Run != nil {
		t.Fatalf("run = %#v, want nil after stale failure", resp.Run)
	}

	_, latestRun, err := repo.ListLatestSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSnapshots() error = %v", err)
	}
	if latestRun == nil || latestRun.Status != "failed" {
		t.Fatalf("latestRun = %#v, want failed", latestRun)
	}
}
