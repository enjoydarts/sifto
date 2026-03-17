package service

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestApplyOpenRouterDescriptionCachePrefillsExistingJapaneseTranslation(t *testing.T) {
	en := "English description"
	ja := "既存の日本語説明"

	models := []repository.OpenRouterModelSnapshot{
		{
			ModelID:       "provider/model-a",
			DescriptionEN: &en,
			DescriptionJA: nil,
		},
	}
	cache := map[string]repository.OpenRouterDescriptionCacheEntry{
		"provider/model-a": {
			ModelID:       "provider/model-a",
			DescriptionEN: &en,
			DescriptionJA: &ja,
		},
	}

	out, missing := ApplyOpenRouterDescriptionCache(models, cache)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want empty", missing)
	}
	if out[0].DescriptionJA == nil || *out[0].DescriptionJA != ja {
		t.Fatalf("description_ja = %v, want %q", out[0].DescriptionJA, ja)
	}
}

func TestApplyOpenRouterDescriptionCacheMarksChangedEnglishDescriptionAsMissing(t *testing.T) {
	newEN := "New description"
	oldEN := "Old description"
	ja := "古い日本語説明"

	models := []repository.OpenRouterModelSnapshot{
		{
			ModelID:       "provider/model-a",
			DescriptionEN: &newEN,
			DescriptionJA: nil,
		},
	}
	cache := map[string]repository.OpenRouterDescriptionCacheEntry{
		"provider/model-a": {
			ModelID:       "provider/model-a",
			DescriptionEN: &oldEN,
			DescriptionJA: &ja,
		},
	}

	out, missing := ApplyOpenRouterDescriptionCache(models, cache)
	if out[0].DescriptionJA != nil {
		t.Fatalf("description_ja = %v, want nil", out[0].DescriptionJA)
	}
	if got := missing["provider/model-a"]; got != newEN {
		t.Fatalf("missing description = %q, want %q", got, newEN)
	}
}
