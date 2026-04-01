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

func TestNormalizeExecutionAttemptsForDetailPreservesPromptMetadata(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	versionID := "pv_123"
	experimentID := "exp_123"
	armID := "arm_123"
	in := []model.ItemLLMExecutionAttempt{
		{
			Model:                 "newer",
			CreatedAt:             base.Add(time.Minute),
			PromptKey:             "summary.default",
			PromptSource:          "template_version",
			PromptVersionID:       &versionID,
			PromptVersionNumber:   ptrInt(3),
			PromptExperimentID:    &experimentID,
			PromptExperimentArmID: &armID,
		},
		{
			Model:        "older",
			CreatedAt:    base,
			PromptKey:    "summary.default",
			PromptSource: "default_code",
		},
	}

	got := normalizeExecutionAttemptsForDetail(in, 0)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[1].PromptSource != "template_version" {
		t.Fatalf("got[1].PromptSource = %q, want template_version", got[1].PromptSource)
	}
	if got[1].PromptVersionNumber == nil || *got[1].PromptVersionNumber != 3 {
		t.Fatalf("got[1].PromptVersionNumber = %v, want 3", got[1].PromptVersionNumber)
	}
	if got[1].PromptExperimentID == nil || *got[1].PromptExperimentID != experimentID {
		t.Fatalf("got[1].PromptExperimentID = %v, want %q", got[1].PromptExperimentID, experimentID)
	}
	if got[1].PromptExperimentArmID == nil || *got[1].PromptExperimentArmID != armID {
		t.Fatalf("got[1].PromptExperimentArmID = %v, want %q", got[1].PromptExperimentArmID, armID)
	}
}

func ptrInt(v int) *int {
	return &v
}
