package repository

import (
	"context"
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
	if _, err := pool.Exec(context.Background(), `TRUNCATE xai_voice_snapshots, xai_voice_sync_runs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate xai voice catalog tables: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestXAIVoiceRepoInsertAndListLatestSnapshots(t *testing.T) {
	db := testDB(t)
	repo := NewXAIVoiceRepo(db)

	runID, err := repo.StartSyncRun(context.Background(), "manual")
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

	runID, err := repo.StartSyncRun(context.Background(), "manual")
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

	runID, err := repo.StartSyncRun(context.Background(), "manual")
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
