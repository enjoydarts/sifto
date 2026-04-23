package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
)

func TestChooseAskModelAcceptsFeatherlessConfiguredModel(t *testing.T) {
	settings := &model.UserSettings{
		AskModel:             strPtr("featherless::Qwen/Qwen3.5-9B"),
		HasFeatherlessAPIKey: true,
	}

	got := chooseAskModel(
		settings,
		false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false,
	)

	if got == nil || *got != "featherless::Qwen/Qwen3.5-9B" {
		t.Fatalf("chooseAskModel(...) = %v, want featherless ask model", got)
	}
}

func TestChooseAskModelAcceptsDeepInfraConfiguredModel(t *testing.T) {
	settings := &model.UserSettings{
		AskModel:           strPtr("deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo"),
		HasDeepInfraAPIKey: true,
	}

	got := chooseAskModel(
		settings,
		false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false,
	)

	if got == nil || *got != "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo" {
		t.Fatalf("chooseAskModel(...) = %v, want deepinfra ask model", got)
	}
}
