package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestOpenRouterTranslationProgressCountsPendingAndCompleted(t *testing.T) {
	ja := "日本語説明"
	en := "English description"

	models := []repository.OpenRouterModelSnapshot{
		{ModelID: "m1", DescriptionEN: &en, DescriptionJA: nil},
		{ModelID: "m2", DescriptionEN: &en, DescriptionJA: &ja},
		{ModelID: "m3", DescriptionEN: nil, DescriptionJA: nil},
		{ModelID: "m4", DescriptionEN: &en, DescriptionJA: &en},
	}

	total, completed := openRouterTranslationProgress(models)
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if completed != 1 {
		t.Fatalf("completed = %d, want 1", completed)
	}
}

func TestOpenRouterPendingTranslationModelsExcludesAlreadyTranslatedEntries(t *testing.T) {
	ja := "日本語説明"
	en := "English description"

	models := []repository.OpenRouterModelSnapshot{
		{ModelID: "m1", DescriptionEN: &en, DescriptionJA: nil},
		{ModelID: "m2", DescriptionEN: &en, DescriptionJA: &ja},
		{ModelID: "m3", DescriptionEN: &en, DescriptionJA: &en},
		{ModelID: "m4", DescriptionEN: nil, DescriptionJA: nil},
	}

	pending := openRouterPendingTranslationModels(models)
	if len(pending) != 2 {
		t.Fatalf("pending len = %d, want 2", len(pending))
	}
	if pending[0].ModelID != "m1" || pending[1].ModelID != "m3" {
		t.Fatalf("pending model ids = [%s, %s], want [m1, m3]", pending[0].ModelID, pending[1].ModelID)
	}
}
