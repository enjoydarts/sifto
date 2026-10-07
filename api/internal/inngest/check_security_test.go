package inngest

import (
	"errors"
	"testing"
)

func TestMalformedFaithfulnessCheckBecomesWarning(t *testing.T) {
	result := fallbackFaithfulnessCheckWarning(errors.New("faithfulness short_comment missing: parse failed"))
	if result == nil || result.Verdict != "warn" {
		t.Fatalf("result=%+v", result)
	}
	if result := fallbackFaithfulnessCheckWarning(errors.New("database unavailable")); result != nil {
		t.Fatal("unrelated errors must propagate")
	}
}
