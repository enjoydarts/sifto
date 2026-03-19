package inngest

import (
	"testing"

	"github.com/enjoydarts/sifto/api/internal/repository"
)

func TestLLMExecutionIdempotencyKeyUsesTriggerID(t *testing.T) {
	triggerA := "trigger-a"
	triggerB := "trigger-b"
	itemID := "item-1"

	base := repository.LLMExecutionEventInput{
		ItemID:       &itemID,
		Provider:     "openrouter",
		Model:        "openrouter::google/gemini-2.5-flash",
		Purpose:      "summary",
		Status:       "success",
		AttemptIndex: 0,
		TriggerID:    &triggerA,
	}

	keyA1 := llmExecutionEventIdempotencyKey(base)
	keyA2 := llmExecutionEventIdempotencyKey(base)
	if keyA1 != keyA2 {
		t.Fatalf("same input produced different keys: %q vs %q", keyA1, keyA2)
	}

	other := base
	other.TriggerID = &triggerB
	keyB := llmExecutionEventIdempotencyKey(other)
	if keyA1 == keyB {
		t.Fatalf("different trigger IDs produced same key: %q", keyA1)
	}
}
