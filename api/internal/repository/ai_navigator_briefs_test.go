package repository

import "testing"

func TestNormalizeAINavigatorBriefText(t *testing.T) {
	t.Parallel()

	invalid := string([]byte{0xe4, 0xb8})
	got := normalizeAINavigatorBriefText("worker failed: " + invalid)
	if got == "worker failed: "+invalid {
		t.Fatalf("normalizeAINavigatorBriefText did not sanitize invalid utf8: %q", got)
	}
	if got == "" {
		t.Fatalf("normalizeAINavigatorBriefText returned empty string")
	}
}
