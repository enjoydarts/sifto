package repository

import (
	"testing"
	"time"
)

func TestCurrentLLMValueMetricsWindowUsesJSTMonthStart(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 30, 0, 0, time.UTC)
	start, end := currentLLMValueMetricsWindow(now)

	if got, want := start.Format("2006-01-02"), "2026-03-01"; got != want {
		t.Fatalf("start = %s, want %s", got, want)
	}
	if got, want := end.Format("2006-01-02"), "2026-03-01"; got != want {
		t.Fatalf("end = %s, want %s", got, want)
	}
}
