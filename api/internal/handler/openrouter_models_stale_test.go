package handler

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestOpenRouterSyncRunIsStale(t *testing.T) {
	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-30 * time.Second)
	old := now.Add(-3 * time.Minute)

	run := &repository.OpenRouterSyncRun{
		Status:         "running",
		LastProgressAt: &recent,
	}
	if openRouterSyncRunIsStale(run, now) {
		t.Fatal("recent progress should not be stale")
	}

	run.LastProgressAt = &old
	if !openRouterSyncRunIsStale(run, now) {
		t.Fatal("old progress should be stale")
	}
}
