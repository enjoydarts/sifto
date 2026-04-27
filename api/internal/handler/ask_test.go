package handler

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestChooseAskModelAcceptsFeatherlessConfiguredModel(t *testing.T) {
	settings := &model.UserSettings{
		AskModel:             strPtr("featherless::Qwen/Qwen3.5-9B"),
		HasFeatherlessAPIKey: true,
	}

	got := chooseAskModel(
		settings,
		false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false,
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
		false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false,
	)

	if got == nil || *got != "deepinfra::meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo" {
		t.Fatalf("chooseAskModel(...) = %v, want deepinfra ask model", got)
	}
}

func TestReorderAskCandidatesUsesRerankOrderAndFallback(t *testing.T) {
	candidates := []model.AskCandidate{
		{Item: model.Item{ID: "item-1"}},
		{Item: model.Item{ID: "item-2"}},
		{Item: model.Item{ID: "item-3"}},
	}

	got := reorderAskCandidates(candidates, []service.AskRerankItem{
		{ItemID: "item-3"},
		{ItemID: "missing"},
		{ItemID: "item-3"},
	}, 2)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "item-3" || got[1].ID != "item-1" {
		t.Fatalf("order = [%s %s], want [item-3 item-1]", got[0].ID, got[1].ID)
	}
}
