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
		false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false,
	)

	if got == nil || *got != "featherless::Qwen/Qwen3.5-9B" {
		t.Fatalf("chooseAskModel(...) = %v, want featherless ask model", got)
	}
}
