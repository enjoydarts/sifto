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

func TestOpenRouterEffectiveAvailabilityAllowsConstrainedOverrideOnly(t *testing.T) {
	constrained := repository.OpenRouterModelSnapshot{
		ModelID:                 "stepfun/step-3.5-flash",
		SupportedParametersJSON: []byte(`["tools"]`),
	}
	removed := repository.OpenRouterModelSnapshot{
		ModelID:                 "stepfun/removed-model",
		SupportedParametersJSON: []byte(`["response_format"]`),
	}

	if got, _ := OpenRouterEffectiveAvailability(constrained, false, false); got != OpenRouterModelConstrained {
		t.Fatalf("constrained without override = %q, want %q", got, OpenRouterModelConstrained)
	}
	if got, _ := OpenRouterEffectiveAvailability(constrained, true, false); got != OpenRouterModelAvailable {
		t.Fatalf("constrained with override = %q, want %q", got, OpenRouterModelAvailable)
	}
	if got, _ := OpenRouterEffectiveAvailability(removed, true, true); got != OpenRouterModelRemoved {
		t.Fatalf("removed with override = %q, want %q", got, OpenRouterModelRemoved)
	}
}

func TestApplyUserOpenRouterOverridesToCatalog(t *testing.T) {
	catalog := &LLMCatalog{
		ChatModels: []LLMModelCatalog{
			{
				ID:       OpenRouterAliasModelID("stepfun/step-3.5-flash"),
				Provider: "openrouter",
				Capabilities: &LLMModelCapabilities{
					SupportsStructuredOutput: false,
				},
			},
			{
				ID:       OpenRouterAliasModelID("openai/gpt-oss-120b"),
				Provider: "openrouter",
				Capabilities: &LLMModelCapabilities{
					SupportsStructuredOutput: true,
				},
			},
		},
	}

	out := ApplyUserOpenRouterOverridesToCatalog(catalog, map[string]repository.OpenRouterModelOverride{
		"stepfun/step-3.5-flash": {
			ModelID:               "stepfun/step-3.5-flash",
			AllowStructuredOutput: true,
		},
	})
	if out.ChatModels[0].Capabilities == nil || !out.ChatModels[0].Capabilities.SupportsStructuredOutput {
		t.Fatalf("overridden model should support structured output")
	}
	if out.ChatModels[1].Capabilities == nil || !out.ChatModels[1].Capabilities.SupportsStructuredOutput {
		t.Fatalf("other openrouter model should remain unchanged")
	}
}
