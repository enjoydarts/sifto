package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeOpenAITTSVoiceFetcher struct {
	voices []repository.OpenAITTSVoiceSnapshot
	err    error
}

func (f *fakeOpenAITTSVoiceFetcher) FetchVoices(_ context.Context) ([]repository.OpenAITTSVoiceSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]repository.OpenAITTSVoiceSnapshot{}, f.voices...), nil
}

func testOpenAITTSVoicesPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	db, err := repository.NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(db.Close)
	lockOpenAITTSVoicesHandlerDB(t, db)
	if _, err := db.Exec(context.Background(), `
		DELETE FROM provider_model_change_events WHERE provider = 'openai';
		DELETE FROM provider_model_snapshots WHERE provider = 'openai';
		DELETE FROM openai_tts_voice_snapshots;
		DELETE FROM openai_tts_voice_sync_runs;
	`); err != nil {
		t.Fatalf("reset openai handler test tables: %v", err)
	}
	return db
}

func lockOpenAITTSVoicesHandlerDB(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231002
	if _, err := db.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}

func TestOpenAITTSVoicesHandlerSync(t *testing.T) {
	db := testOpenAITTSVoicesPool(t)
	handler := NewOpenAITTSVoicesHandler(
		repository.NewOpenAITTSVoiceRepo(db),
		repository.NewProviderModelUpdateRepo(db),
		&fakeOpenAITTSVoiceFetcher{voices: []repository.OpenAITTSVoiceSnapshot{
			{
				VoiceID:      "alloy",
				Name:         "Alloy",
				Description:  "OpenAI built-in voice",
				Language:     "multilingual",
				MetadataJSON: mustOpenAITTSJSON(t, map[string]any{"voice_kind": "builtin"}),
			},
		}},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/openai-tts-voices/sync", nil)
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		LatestRun *repository.OpenAITTSVoiceSyncRun   `json:"latest_run"`
		Voices    []repository.OpenAITTSVoiceSnapshot `json:"voices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LatestRun == nil || resp.LatestRun.Status != "success" {
		t.Fatalf("latest_run = %#v, want success", resp.LatestRun)
	}
	if len(resp.Voices) != 1 || resp.Voices[0].VoiceID != "alloy" {
		t.Fatalf("voices = %#v, want alloy", resp.Voices)
	}

	snapshot, err := repository.NewProviderModelUpdateRepo(db).GetSnapshot(context.Background(), "openai")
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	if snapshot == nil || len(snapshot.Models) != 1 || snapshot.Models[0] != "alloy" {
		t.Fatalf("snapshot = %#v, want openai alloy", snapshot)
	}
}

func TestOpenAITTSVoicesHandlerSyncFailurePreservesBaseline(t *testing.T) {
	db := testOpenAITTSVoicesPool(t)
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	if err := updateRepo.UpsertSnapshot(context.Background(), "openai", []string{"voice-existing"}, "ok", nil); err != nil {
		t.Fatalf("UpsertSnapshot() error = %v", err)
	}

	handler := NewOpenAITTSVoicesHandler(
		repository.NewOpenAITTSVoiceRepo(db),
		updateRepo,
		&fakeOpenAITTSVoiceFetcher{err: errors.New("upstream failed")},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/openai-tts-voices/sync", nil)
	rec := httptest.NewRecorder()

	handler.Sync(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 body=%s", rec.Code, rec.Body.String())
	}

	snapshot, err := updateRepo.GetSnapshot(context.Background(), "openai")
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

func TestOpenAITTSVoicesHandlerListUsesLastSuccessfulBaselineAfterFailedLatestRun(t *testing.T) {
	db := testOpenAITTSVoicesPool(t)
	repo := repository.NewOpenAITTSVoiceRepo(db)

	successRunID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun(success) error = %v", err)
	}
	if err := repo.InsertSnapshots(context.Background(), successRunID, time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC), []repository.OpenAITTSVoiceSnapshot{
		{VoiceID: "alloy", Name: "Alloy", Description: "OpenAI built-in voice", Language: "multilingual"},
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

	handler := &OpenAITTSVoicesHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/openai-tts-voices", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		LatestRun *repository.OpenAITTSVoiceSyncRun   `json:"latest_run"`
		Voices    []repository.OpenAITTSVoiceSnapshot `json:"voices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.LatestRun == nil || resp.LatestRun.ID != successRunID || resp.LatestRun.Status != "success" {
		t.Fatalf("latest_run = %#v, want success run %d", resp.LatestRun, successRunID)
	}
	if len(resp.Voices) != 1 || resp.Voices[0].VoiceID != "alloy" {
		t.Fatalf("voices = %#v, want alloy", resp.Voices)
	}
}

func TestOpenAITTSVoicesHandlerStatusReturnsRunningRun(t *testing.T) {
	db := testOpenAITTSVoicesPool(t)

	repo := repository.NewOpenAITTSVoiceRepo(db)
	runID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}

	handler := &OpenAITTSVoicesHandler{repo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/openai-tts-voices/status", nil)
	rec := httptest.NewRecorder()

	handler.Status(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Run *repository.OpenAITTSVoiceSyncRun `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Run == nil || resp.Run.ID != runID || resp.Run.Status != "running" {
		t.Fatalf("run = %#v, want running run %d", resp.Run, runID)
	}
}

func mustOpenAITTSJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}
