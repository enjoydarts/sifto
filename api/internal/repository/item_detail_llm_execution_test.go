package repository

import (
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestNormalizeExecutionAttemptsForDetail(t *testing.T) {
	base := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	in := []model.ItemLLMExecutionAttempt{
		{Model: "m4", CreatedAt: base.Add(4 * time.Minute)},
		{Model: "m3", CreatedAt: base.Add(3 * time.Minute)},
		{Model: "m2", CreatedAt: base.Add(2 * time.Minute)},
		{Model: "m1", CreatedAt: base.Add(1 * time.Minute)},
		{Model: "m0", CreatedAt: base},
	}

	got := normalizeExecutionAttemptsForDetail(in, 4)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}

	want := []string{"m1", "m2", "m3", "m4"}
	for i, modelID := range want {
		if got[i].Model != modelID {
			t.Fatalf("got[%d].Model = %q, want %q", i, got[i].Model, modelID)
		}
	}
}

func TestNormalizeExecutionAttemptsForDetailWithoutLimit(t *testing.T) {
	base := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	in := []model.ItemLLMExecutionAttempt{
		{Model: "m3", CreatedAt: base.Add(3 * time.Minute)},
		{Model: "m2", CreatedAt: base.Add(2 * time.Minute)},
		{Model: "m1", CreatedAt: base.Add(1 * time.Minute)},
	}

	got := normalizeExecutionAttemptsForDetail(in, 0)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	want := []string{"m1", "m2", "m3"}
	for i, modelID := range want {
		if got[i].Model != modelID {
			t.Fatalf("got[%d].Model = %q, want %q", i, got[i].Model, modelID)
		}
	}
}
