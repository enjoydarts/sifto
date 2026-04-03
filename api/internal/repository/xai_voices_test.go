package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockXAIVoiceTestDB(t, pool)
	if _, err := pool.Exec(context.Background(), `
		DELETE FROM xai_voice_snapshots;
		DELETE FROM xai_voice_sync_runs;
	`); err != nil {
		t.Fatalf("reset xai voice catalog tables: %v", err)
	}
	return pool
}

func lockXAIVoiceTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231001
	if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}

func testTriggerType(t *testing.T, name string) string {
	t.Helper()
	return fmt.Sprintf("repo-test-%s-%d", name, time.Now().UnixNano())
}

func TestXAIVoiceRepoInsertAndListLatestSnapshots(t *testing.T) {
	db := testDB(t)
	repo := NewXAIVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), testTriggerType(t, "insert-list"))
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	fetchedAt := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	err = repo.InsertSnapshots(context.Background(), runID, fetchedAt, []XAIVoiceSnapshot{
		{VoiceID: "voice-1", Name: "Grok Voice 1", Description: "Warm", Language: "en", PreviewURL: "https://example.com/1.mp3"},
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
	if len(rows) != 1 || rows[0].VoiceID != "voice-1" {
		t.Fatalf("rows = %#v, want voice-1", rows)
	}
}

func TestXAIVoiceRepoFinishSyncRunMarksFailedWhenErrorMessagePresent(t *testing.T) {
	db := testDB(t)
	repo := NewXAIVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), testTriggerType(t, "finish-failed"))
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	errMsg := "xai voice sync failed"
	if err := repo.FinishSyncRun(context.Background(), runID, 2, 0, &errMsg); err != nil {
		t.Fatalf("FinishSyncRun() error = %v", err)
	}

	_, latestRun, err := repo.ListLatestSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSnapshots() error = %v", err)
	}
	if latestRun == nil {
		t.Fatal("latestRun = nil, want failed run")
	}
	if latestRun.Status != "failed" {
		t.Fatalf("latestRun.Status = %q, want failed", latestRun.Status)
	}
	if latestRun.ErrorMessage == nil || *latestRun.ErrorMessage != errMsg {
		t.Fatalf("latestRun.ErrorMessage = %v, want %q", latestRun.ErrorMessage, errMsg)
	}
}

func TestXAIVoiceRepoFailSyncRunMarksFailedAndStoresErrorMessage(t *testing.T) {
	db := testDB(t)
	repo := NewXAIVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), testTriggerType(t, "fail-sync"))
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	if err := repo.FailSyncRun(context.Background(), runID, "timeout while fetching voices"); err != nil {
		t.Fatalf("FailSyncRun() error = %v", err)
	}

	_, latestRun, err := repo.ListLatestSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSnapshots() error = %v", err)
	}
	if latestRun == nil {
		t.Fatal("latestRun = nil, want failed run")
	}
	if latestRun.Status != "failed" {
		t.Fatalf("latestRun.Status = %q, want failed", latestRun.Status)
	}
	if latestRun.ErrorMessage == nil || *latestRun.ErrorMessage != "timeout while fetching voices" {
		t.Fatalf("latestRun.ErrorMessage = %v, want timeout while fetching voices", latestRun.ErrorMessage)
	}
}

func TestXAIVoiceRepoListLatestSuccessfulSnapshotsFallsBackFromFailedLatestRun(t *testing.T) {
	db := testDB(t)
	repo := NewXAIVoiceRepo(db)

	successRunID, err := repo.StartSyncRun(context.Background(), testTriggerType(t, "success"))
	if err != nil {
		t.Fatalf("StartSyncRun(success) error = %v", err)
	}
	fetchedAt := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	if err := repo.InsertSnapshots(context.Background(), successRunID, fetchedAt, []XAIVoiceSnapshot{
		{VoiceID: "voice-1", Name: "Baseline Voice", Description: "Warm", Language: "en"},
	}); err != nil {
		t.Fatalf("InsertSnapshots(success) error = %v", err)
	}
	if err := repo.FinishSyncRun(context.Background(), successRunID, 1, 1, nil); err != nil {
		t.Fatalf("FinishSyncRun(success) error = %v", err)
	}

	failedRunID, err := repo.StartSyncRun(context.Background(), testTriggerType(t, "failed"))
	if err != nil {
		t.Fatalf("StartSyncRun(failed) error = %v", err)
	}
	errMsg := "upstream failed"
	if err := repo.FinishSyncRun(context.Background(), failedRunID, 0, 0, &errMsg); err != nil {
		t.Fatalf("FinishSyncRun(failed) error = %v", err)
	}

	rows, run, err := repo.ListLatestSuccessfulSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListLatestSuccessfulSnapshots() error = %v", err)
	}
	if run == nil || run.ID != successRunID {
		t.Fatalf("run = %#v, want success run %d", run, successRunID)
	}
	if len(rows) != 1 || rows[0].VoiceID != "voice-1" {
		t.Fatalf("rows = %#v, want preserved voice-1 baseline", rows)
	}
}
