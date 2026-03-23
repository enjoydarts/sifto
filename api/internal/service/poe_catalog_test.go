package service

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestApplyPoeDescriptionCachePrefillsExistingJapaneseTranslation(t *testing.T) {
	en := "English description"
	ja := "既存の日本語説明"

	models := []repository.PoeModelSnapshot{
		{
			ModelID:       "Claude-Sonnet-4.5",
			DescriptionEN: &en,
			DescriptionJA: nil,
		},
	}
	cache := map[string]repository.PoeDescriptionCacheEntry{
		"Claude-Sonnet-4.5": {
			ModelID:       "Claude-Sonnet-4.5",
			DescriptionEN: &en,
			DescriptionJA: &ja,
		},
	}

	out, missing := ApplyPoeDescriptionCache(models, cache)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want empty", missing)
	}
	if out[0].DescriptionJA == nil || *out[0].DescriptionJA != ja {
		t.Fatalf("description_ja = %v, want %q", out[0].DescriptionJA, ja)
	}
}

func TestApplyPoeDescriptionCacheMarksChangedEnglishDescriptionAsMissing(t *testing.T) {
	newEN := "New description"
	oldEN := "Old description"
	ja := "古い日本語説明"

	models := []repository.PoeModelSnapshot{
		{
			ModelID:       "Claude-Sonnet-4.5",
			DescriptionEN: &newEN,
			DescriptionJA: nil,
		},
	}
	cache := map[string]repository.PoeDescriptionCacheEntry{
		"Claude-Sonnet-4.5": {
			ModelID:       "Claude-Sonnet-4.5",
			DescriptionEN: &oldEN,
			DescriptionJA: &ja,
		},
	}

	out, missing := ApplyPoeDescriptionCache(models, cache)
	if out[0].DescriptionJA != nil {
		t.Fatalf("description_ja = %v, want nil", out[0].DescriptionJA)
	}
	if got := missing["Claude-Sonnet-4.5"]; got != newEN {
		t.Fatalf("missing description = %q, want %q", got, newEN)
	}
}
