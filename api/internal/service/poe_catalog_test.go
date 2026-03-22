package service

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestPoePreferredTransportPrefersAnthropicForOfficialClaudeModels(t *testing.T) {
	model := repository.PoeModelSnapshot{
		ModelID:  "Claude-Sonnet-4.5",
		OwnedBy:  "Anthropic",
		IsActive: true,
	}

	if got := PoePreferredTransport(model); got != "anthropic" {
		t.Fatalf("PoePreferredTransport() = %q, want %q", got, "anthropic")
	}
	if !PoeSupportsAnthropicCompat(model) {
		t.Fatal("Anthropic Claude model should support anthropic compat")
	}
}

func TestPoePreferredTransportFallsBackToOpenAIForNonClaudeModels(t *testing.T) {
	model := repository.PoeModelSnapshot{
		ModelID:  "GPT-5",
		OwnedBy:  "OpenAI",
		IsActive: true,
	}

	if got := PoePreferredTransport(model); got != "openai" {
		t.Fatalf("PoePreferredTransport() = %q, want %q", got, "openai")
	}
	if PoeSupportsAnthropicCompat(model) {
		t.Fatal("non-Claude model should not support anthropic compat")
	}
}

func TestPoeSnapshotsToCatalogModelsUsesPoeAliasAndSnapshotPricing(t *testing.T) {
	description := "Model description"
	models := []repository.PoeModelSnapshot{
		{
			ModelID:                        "Claude-Sonnet-4.5",
			DisplayName:                    "Claude-Sonnet-4.5",
			OwnedBy:                        "Anthropic",
			DescriptionEN:                  &description,
			PricingJSON:                    []byte(`{"prompt":"0.000003","completion":"0.000015"}`),
			TransportSupportsOpenAICompat:  true,
			TransportSupportsAnthropicCompat: true,
			PreferredTransport:             "anthropic",
			IsActive:                       true,
		},
	}

	out := PoeSnapshotsToCatalogModels(models)
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].ID != PoeAliasModelID("Claude-Sonnet-4.5") {
		t.Fatalf("id = %q", out[0].ID)
	}
	if out[0].Provider != "poe" {
		t.Fatalf("provider = %q, want poe", out[0].Provider)
	}
	if out[0].Pricing == nil || out[0].Pricing.PricingSource != "poe_snapshot" {
		t.Fatalf("pricing = %#v, want poe_snapshot", out[0].Pricing)
	}
}
