package handler

import (
	"testing"
	"time"
)

func TestBuildBriefingNavigatorIntroContext(t *testing.T) {
	now := time.Date(2026, 3, 23, 19, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	got := buildBriefingNavigatorIntroContext(now)

	if got.TimeOfDay != "evening" {
		t.Fatalf("time_of_day = %q", got.TimeOfDay)
	}
	if got.WeekdayJST != "Monday" {
		t.Fatalf("weekday_jst = %q", got.WeekdayJST)
	}
	if got.SeasonHint == "" {
		t.Fatal("season_hint is empty")
	}
	if got.NowJST == "" || got.DateJST == "" {
		t.Fatalf("missing now/date: %+v", got)
	}
}

func TestCacheKeyBriefingNavigatorVariesByPersonaModelAndPreview(t *testing.T) {
	k1 := cacheKeyBriefingNavigator("u1", "editor", "gpt-5", false)
	k2 := cacheKeyBriefingNavigator("u1", "snark", "gpt-5", false)
	k3 := cacheKeyBriefingNavigator("u1", "editor", "gpt-5-mini", false)
	k4 := cacheKeyBriefingNavigator("u1", "editor", "gpt-5", true)

	if k1 == k2 || k1 == k3 || k1 == k4 {
		t.Fatalf("cache key should vary, got k1=%q k2=%q k3=%q k4=%q", k1, k2, k3, k4)
	}
}
