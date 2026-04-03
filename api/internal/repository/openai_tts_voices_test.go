package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOpenAITTSVoiceRepoInsertAndListLatestSnapshots(t *testing.T) {
	db := testOpenAITTSDB(t)
	repo := NewOpenAITTSVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	fetchedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	err = repo.InsertSnapshots(context.Background(), runID, fetchedAt, []OpenAITTSVoiceSnapshot{
		{
			VoiceID:      "alloy",
			Name:         "Alloy",
			Description:  "OpenAI built-in voice",
			Language:     "multilingual",
			MetadataJSON: mustOpenAITTSJSON(t, map[string]any{"voice_kind": "builtin"}),
		},
	})
	if err != nil {
		t.Fatalf("InsertSnapshots() error = %v", err)
	}
	if err := repo.FinishSyncRun(context.Background(), runID, 1, 1, nil); err != nil {
		t.Fatalf("FinishSyncRun() error = %v", err)
	}

	rows, latestRun, err := repo.ListLatestSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSnapshots() error = %v", err)
	}
	if latestRun == nil || latestRun.Status != "success" {
		t.Fatalf("latestRun = %#v, want success run", latestRun)
	}
	if len(rows) != 1 || rows[0].VoiceID != "alloy" {
		t.Fatalf("rows = %#v, want alloy", rows)
	}
}

func TestOpenAITTSVoiceRepoListLatestSuccessfulSnapshotsSkipsFailedLatestRun(t *testing.T) {
	db := testOpenAITTSDB(t)
	repo := NewOpenAITTSVoiceRepo(db)

	successRunID, err := repo.StartSyncRun(context.Background(), "manual")
	if err != nil {
		t.Fatalf("StartSyncRun(success) error = %v", err)
	}
	if err := repo.InsertSnapshots(context.Background(), successRunID, time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC), []OpenAITTSVoiceSnapshot{
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

	rows, latestRun, err := repo.ListLatestSuccessfulSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSuccessfulSnapshots() error = %v", err)
	}
	if latestRun == nil || latestRun.ID != successRunID || latestRun.Status != "success" {
		t.Fatalf("latestRun = %#v, want success run %d", latestRun, successRunID)
	}
	if len(rows) != 1 || rows[0].VoiceID != "alloy" {
		t.Fatalf("rows = %#v, want alloy", rows)
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

func testOpenAITTSDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockOpenAITTSRepoTestDB(t, pool)
	if _, err := pool.Exec(context.Background(), `
		DELETE FROM openai_tts_voice_snapshots;
		DELETE FROM openai_tts_voice_sync_runs;
	`); err != nil {
		t.Fatalf("reset openai tts voice catalog tables: %v", err)
	}
	return pool
}

func lockOpenAITTSRepoTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231003
	if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}
