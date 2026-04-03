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
