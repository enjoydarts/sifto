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
	"github.com/jackc/pgx/v5/pgxpool"
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

func testXAIVoicesPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockXAIVoicesHandlerDB(t, db)
	if _, err := db.Exec(context.Background(), `
		DELETE FROM provider_model_change_events WHERE provider = 'xai';
		DELETE FROM provider_model_snapshots WHERE provider = 'xai';
		DELETE FROM xai_voice_snapshots;
		DELETE FROM xai_voice_sync_runs;
		DELETE FROM user_settings WHERE user_id IN (
			'00000000-0000-4000-8000-000000000001',
			'00000000-0000-4000-8000-000000000002',
			'00000000-0000-4000-8000-000000000003',
			'00000000-0000-4000-8000-000000000004',
			'00000000-0000-4000-8000-000000000005'
		);
		DELETE FROM users WHERE id IN (
			'00000000-0000-4000-8000-000000000001',
			'00000000-0000-4000-8000-000000000002',
			'00000000-0000-4000-8000-000000000003',
			'00000000-0000-4000-8000-000000000004',
			'00000000-0000-4000-8000-000000000005'
		);
	`); err != nil {
		t.Fatalf("reset xai handler test tables: %v", err)
	}
	return db
}

func lockXAIVoicesHandlerDB(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231001
	if _, err := db.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}

func TestXAIVoicesHandlerSync(t *testing.T) {
	const userID = "00000000-0000-4000-8000-000000000001"

	db := testXAIVoicesPool(t)
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

	db := testXAIVoicesPool(t)
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

	db := testXAIVoicesPool(t)
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
	if snapshot.Status != "failed" {
		t.Fatalf("snapshot.Status = %q, want failed", snapshot.Status)
	}
	if snapshot.Error == nil || *snapshot.Error != "upstream failed" {
		t.Fatalf("snapshot.Error = %v, want upstream failed", snapshot.Error)
	}
}

func TestXAIVoicesHandlerSyncInsertFailurePreservesExistingXAIBaselineAndMarksFailed(t *testing.T) {
	const userID = "00000000-0000-4000-8000-000000000005"

	db := testXAIVoicesPool(t)
	if _, err := db.Exec(context.Background(), `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, userID, "xai-insert-fail@example.com", "xAI Insert Fail"); err != nil {
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
		&fakeXAIVoiceFetcher{voices: []repository.XAIVoiceSnapshot{
			{VoiceID: "dup-voice", Name: "One", Description: "Warm", Language: "en"},
			{VoiceID: "dup-voice", Name: "Two", Description: "Bright", Language: "en"},
		}},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/xai-voices/sync", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 body=%s", rec.Code, rec.Body.String())
	}

	snapshot, err := updateRepo.GetSnapshot(context.Background(), "xai")
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	if snapshot == nil || len(snapshot.Models) != 1 || snapshot.Models[0] != "voice-existing" {
		t.Fatalf("snapshot = %#v, want preserved voice-existing baseline", snapshot)
	}
	if snapshot.Status != "failed" {
		t.Fatalf("snapshot.Status = %q, want failed", snapshot.Status)
	}
	if snapshot.Error == nil || !strings.Contains(*snapshot.Error, "duplicate key value") {
		t.Fatalf("snapshot.Error = %v, want duplicate key error", snapshot.Error)
	}
}

func TestXAIVoicesHandlerListUsesLastSuccessfulBaselineAfterFailedLatestRun(t *testing.T) {
	db := testXAIVoicesPool(t)
	repo := repository.NewXAIVoiceRepo(db)

	successRunID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun(success) error = %v", err)
	}
	fetchedAt := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	if err := repo.InsertSnapshots(context.Background(), successRunID, fetchedAt, []repository.XAIVoiceSnapshot{
		{VoiceID: "voice-1", Name: "Baseline Voice", Description: "Warm", Language: "en"},
	}); err != nil {
		t.Fatalf("InsertSnapshots(success) error = %v", err)
	}
	if err := repo.FinishSyncRun(context.Background(), successRunID, 1, 1, nil); err != nil {
		t.Fatalf("FinishSyncRun(success) error = %v", err)
	}

	failedRunID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun(failed) error = %v", err)
	}
	errMsg := "upstream failed"
	if err := repo.FinishSyncRun(context.Background(), failedRunID, 0, 0, &errMsg); err != nil {
		t.Fatalf("FinishSyncRun(failed) error = %v", err)
	}

	handler := &XAIVoicesHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/xai-voices", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		LatestRun *repository.XAIVoiceSyncRun   `json:"latest_run"`
		Voices    []repository.XAIVoiceSnapshot `json:"voices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LatestRun == nil || resp.LatestRun.ID != failedRunID || resp.LatestRun.Status != "failed" {
		t.Fatalf("latest_run = %#v, want failed latest run %d", resp.LatestRun, failedRunID)
	}
	if len(resp.Voices) != 1 || resp.Voices[0].VoiceID != "voice-1" {
		t.Fatalf("voices = %#v, want preserved successful baseline", resp.Voices)
	}
}

func TestXAIVoicesHandlerSyncExposesLatestChangeSummary(t *testing.T) {
	const userID = "00000000-0000-4000-8000-000000000001"

	db := testXAIVoicesPool(t)
	if _, err := db.Exec(context.Background(), `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, userID, "xai-changes@example.com", "xAI Changes"); err != nil {
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
	if err := updateRepo.UpsertSnapshot(context.Background(), "xai", []string{"voice-old"}, "ok", nil); err != nil {
		t.Fatalf("UpsertSnapshot() error = %v", err)
	}

	handler := NewXAIVoicesHandler(
		repository.NewXAIVoiceRepo(db),
		settingsRepo,
		updateRepo,
		cipher,
		&fakeXAIVoiceFetcher{voices: []repository.XAIVoiceSnapshot{
			{VoiceID: "voice-new", Name: "New Voice", Description: "Warm", Language: "en"},
		}},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/xai-voices/sync", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		LatestRun           *repository.XAIVoiceSyncRun `json:"latest_run"`
		LatestChangeSummary *struct {
			Provider string `json:"provider"`
			Trigger  string `json:"trigger"`
			Added    []struct {
				ModelID string `json:"model_id"`
			} `json:"added"`
			Removed []struct {
				ModelID string `json:"model_id"`
			} `json:"removed"`
		} `json:"latest_change_summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LatestRun == nil || resp.LatestRun.Status != "success" {
		t.Fatalf("latest_run = %#v, want success", resp.LatestRun)
	}
	if resp.LatestChangeSummary == nil {
		t.Fatal("latest_change_summary = nil, want summary")
	}
	if resp.LatestChangeSummary.Provider != "xai" {
		t.Fatalf("summary.Provider = %q, want xai", resp.LatestChangeSummary.Provider)
	}
	if resp.LatestChangeSummary.Trigger != "manual" {
		t.Fatalf("summary.Trigger = %q, want manual", resp.LatestChangeSummary.Trigger)
	}
	if len(resp.LatestChangeSummary.Added) != 1 || resp.LatestChangeSummary.Added[0].ModelID != "voice-new" {
		t.Fatalf("summary.Added = %#v, want voice-new", resp.LatestChangeSummary.Added)
	}
	if len(resp.LatestChangeSummary.Removed) != 1 || resp.LatestChangeSummary.Removed[0].ModelID != "voice-old" {
		t.Fatalf("summary.Removed = %#v, want voice-old", resp.LatestChangeSummary.Removed)
	}
}

func TestXAIVoicesHandlerSyncSecretLoadInternalFailureIsNotReportedAsMissingKey(t *testing.T) {
	db := testXAIVoicesPool(t)

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
	db := testXAIVoicesPool(t)

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
	db := testXAIVoicesPool(t)

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
