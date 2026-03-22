package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestPoeTranslationProgressCountsPendingAndCompleted(t *testing.T) {
	ja := "日本語説明"
	en := "English description"

	models := []repository.PoeModelSnapshot{
		{ModelID: "m1", DescriptionEN: &en, DescriptionJA: nil},
		{ModelID: "m2", DescriptionEN: &en, DescriptionJA: &ja},
		{ModelID: "m3", DescriptionEN: nil, DescriptionJA: nil},
		{ModelID: "m4", DescriptionEN: &en, DescriptionJA: &en},
	}

	total, completed := poeTranslationProgress(models)
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if completed != 1 {
		t.Fatalf("completed = %d, want 1", completed)
	}
}

func TestBuildPoeRecentChanges(t *testing.T) {
	prev := []repository.PoeModelSnapshot{
		{ModelID: "kept"},
		{ModelID: "removed"},
	}
	curr := []repository.PoeModelSnapshot{
		{ModelID: "kept"},
		{ModelID: "added"},
	}

	got := buildPoeRecentChanges(prev, curr)
	if got["added"] != "added" {
		t.Fatalf("added change = %q, want added", got["added"])
	}
	if got["removed"] != "removed" {
		t.Fatalf("removed change = %q, want removed", got["removed"])
	}
	if _, exists := got["kept"]; exists {
		t.Fatal("kept model should not be marked as changed")
	}
}
