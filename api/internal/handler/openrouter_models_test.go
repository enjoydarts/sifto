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
