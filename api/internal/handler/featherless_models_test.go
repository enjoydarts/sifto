package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestBuildFeatherlessRecentChanges(t *testing.T) {
	prev := []repository.FeatherlessModelSnapshot{
		{ModelID: "kept", AvailableOnCurrentPlan: true, IsGated: false},
		{ModelID: "availability-changed", AvailableOnCurrentPlan: true, IsGated: false},
		{ModelID: "gated-changed", AvailableOnCurrentPlan: true, IsGated: false},
		{ModelID: "removed", AvailableOnCurrentPlan: true, IsGated: false},
	}
	curr := []repository.FeatherlessModelSnapshot{
		{ModelID: "kept", AvailableOnCurrentPlan: true, IsGated: false},
		{ModelID: "added", AvailableOnCurrentPlan: true, IsGated: false},
		{ModelID: "availability-changed", AvailableOnCurrentPlan: false, IsGated: false},
		{ModelID: "gated-changed", AvailableOnCurrentPlan: true, IsGated: true},
	}

	got := buildFeatherlessRecentChanges(prev, curr)
	if got["added"] != "added" {
		t.Fatalf("added = %q, want added", got["added"])
	}
	if got["availability-changed"] != "availability_changed" {
		t.Fatalf("availability-changed = %q, want availability_changed", got["availability-changed"])
	}
	if got["gated-changed"] != "gated_changed" {
		t.Fatalf("gated-changed = %q, want gated_changed", got["gated-changed"])
	}
	if got["removed"] != "removed" {
		t.Fatalf("removed = %q, want removed", got["removed"])
	}
	if _, exists := got["kept"]; exists {
		t.Fatal("kept model should not be marked as changed")
	}
}

func TestSplitFeatherlessModelEntriesPutsPlanUnavailableAndRemovedIntoUnavailableList(t *testing.T) {
	current := []repository.FeatherlessModelSnapshot{
		{ModelID: "available", DisplayName: "Available", AvailableOnCurrentPlan: true},
		{ModelID: "plan-blocked", DisplayName: "Plan Blocked", AvailableOnCurrentPlan: false},
	}
	previous := []repository.FeatherlessModelSnapshot{
		{ModelID: "removed", DisplayName: "Removed", AvailableOnCurrentPlan: true},
	}

	available, unavailable := splitFeatherlessModelEntries(current, previous, map[string]string{
		"plan-blocked": "availability_changed",
		"removed":      "removed",
	})
	if len(available) != 1 || available[0].ModelID != "available" {
		t.Fatalf("available = %#v, want only available model", available)
	}
	if len(unavailable) != 2 {
		t.Fatalf("unavailable len = %d, want 2", len(unavailable))
	}
	if unavailable[0].ModelID != "plan-blocked" {
		t.Fatalf("unavailable[0].model_id = %q, want %q", unavailable[0].ModelID, "plan-blocked")
	}
	if unavailable[1].ModelID != "removed" {
		t.Fatalf("unavailable[1].model_id = %q, want %q", unavailable[1].ModelID, "removed")
	}
}
