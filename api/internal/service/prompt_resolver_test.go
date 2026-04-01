package service

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestChoosePromptExperimentArmDeterministic(t *testing.T) {
	t.Parallel()

	arms := []repository.PromptExperimentArm{
		{ID: "arm-a", Weight: 30},
		{ID: "arm-b", Weight: 70},
	}

	first := choosePromptExperimentArm(arms, "item-1", "exp-1")
	second := choosePromptExperimentArm(arms, "item-1", "exp-1")
	if first == nil || second == nil {
		t.Fatal("expected arm to be selected")
	}
	if first.ID != second.ID {
		t.Fatalf("selection must be deterministic: %s vs %s", first.ID, second.ID)
	}
}

func TestChoosePromptExperimentArmRejectsMissingAssignmentKey(t *testing.T) {
	t.Parallel()

	arms := []repository.PromptExperimentArm{{ID: "arm-a", Weight: 100}}
	if got := choosePromptExperimentArm(arms, "", "exp-1"); got != nil {
		t.Fatalf("expected nil arm for empty assignment key, got %+v", got)
	}
}
