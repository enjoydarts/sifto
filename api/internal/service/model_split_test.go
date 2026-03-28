package service

import "testing"

func TestNormalizeModelSplitRatePercent(t *testing.T) {
	if got := normalizeModelSplitRatePercent(nil); got != 0 {
		t.Fatalf("normalizeModelSplitRatePercent(nil) = %d, want 0", got)
	}
	if got := normalizeModelSplitRatePercent(intPtr(-10)); got != 0 {
		t.Fatalf("normalizeModelSplitRatePercent(-10) = %d, want 0", got)
	}
	if got := normalizeModelSplitRatePercent(intPtr(25)); got != 25 {
		t.Fatalf("normalizeModelSplitRatePercent(25) = %d, want 25", got)
	}
	if got := normalizeModelSplitRatePercent(intPtr(120)); got != 100 {
		t.Fatalf("normalizeModelSplitRatePercent(120) = %d, want 100", got)
	}
}

func TestResolveSplitPrimaryModel(t *testing.T) {
	primary := "gpt-5.4-mini"
	secondary := "google/gemini-2.5-flash"

	if got := resolveSplitPrimaryModel(&primary, nil, 50, func(int) int { return 0 }); got == nil || *got != primary {
		t.Fatalf("resolveSplitPrimaryModel(primary only) = %v, want %q", got, primary)
	}
	if got := resolveSplitPrimaryModel(&primary, &secondary, 0, func(int) int { return 0 }); got == nil || *got != primary {
		t.Fatalf("resolveSplitPrimaryModel(rate 0) = %v, want %q", got, primary)
	}
	if got := resolveSplitPrimaryModel(&primary, &secondary, 100, func(int) int { return 99 }); got == nil || *got != secondary {
		t.Fatalf("resolveSplitPrimaryModel(rate 100) = %v, want %q", got, secondary)
	}
	if got := resolveSplitPrimaryModel(&primary, &secondary, 33, func(int) int { return 32 }); got == nil || *got != secondary {
		t.Fatalf("resolveSplitPrimaryModel(draw 32) = %v, want %q", got, secondary)
	}
	if got := resolveSplitPrimaryModel(&primary, &secondary, 33, func(int) int { return 33 }); got == nil || *got != primary {
		t.Fatalf("resolveSplitPrimaryModel(draw 33) = %v, want %q", got, primary)
	}
}

func intPtr(v int) *int { return &v }
