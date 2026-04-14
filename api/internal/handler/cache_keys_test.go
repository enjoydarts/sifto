package handler

import (
	"strings"
	"testing"
)

func TestCacheKeyVersionKeys(t *testing.T) {
	if got, want := cacheVersionKeyUserItems("u1"), "cache_version:user_items:u1"; got != want {
		t.Fatalf("cacheVersionKeyUserItems = %q, want %q", got, want)
	}
	if got, want := cacheVersionKeyItemDetail("item-1"), "cache_version:item_detail:v2:item-1"; got != want {
		t.Fatalf("cacheVersionKeyItemDetail = %q, want %q", got, want)
	}
	if got, want := cacheVersionKeyUserSettings("u1"), "cache_version:user_settings:u1"; got != want {
		t.Fatalf("cacheVersionKeyUserSettings = %q, want %q", got, want)
	}
	if got, want := cacheVersionKeyUserPreferenceProfile("u1"), "cache_version:user_preference_profile:u1"; got != want {
		t.Fatalf("cacheVersionKeyUserPreferenceProfile = %q, want %q", got, want)
	}
	if got, want := cacheVersionKeyUserLLMUsage("u1"), "cache_version:user_llm_usage:u1"; got != want {
		t.Fatalf("cacheVersionKeyUserLLMUsage = %q, want %q", got, want)
	}
}

func TestCacheKeyItemsListVersioned(t *testing.T) {
	got := cacheKeyItemsListVersioned("u1", 7, "summarized", "src-1", "go", "openai", "and", true, false, true, false, "score", 2, 50)
	wantParts := []string{
		"v1:items:list:u1:v=7",
		"status=summarized",
		"source=src-1",
		"topic=go",
		"q=openai",
		"mode=and",
		"unread=true",
		"read=false",
		"fav=true",
		"later=false",
		"sort=score",
		"page=2",
		"size=50",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("cacheKeyItemsListVersioned missing %q in %q", part, got)
		}
	}
}

func TestCacheKeyItemDetailVersioned(t *testing.T) {
	got := cacheKeyItemDetailVersioned("u1", "item-1", 3)
	want := "v1:items:detail:u1:item=item-1:v=3"
	if got != want {
		t.Fatalf("cacheKeyItemDetailVersioned = %q, want %q", got, want)
	}
}

func TestCacheKeySettingsGetVersioned(t *testing.T) {
	got := cacheKeySettingsGetVersioned("u1", 9)
	want := "v1:settings:get:u1:v=9"
	if got != want {
		t.Fatalf("cacheKeySettingsGetVersioned = %q, want %q", got, want)
	}

	if got, want = cacheKeyPreferenceProfile("u1", 3), "v1:settings:preference_profile:u1:v=3"; got != want {
		t.Fatalf("cacheKeyPreferenceProfile = %q, want %q", got, want)
	}
	if got, want = cacheKeyPreferenceProfileSummary("u1", 4), "v1:settings:preference_profile_summary:u1:v=4"; got != want {
		t.Fatalf("cacheKeyPreferenceProfileSummary = %q, want %q", got, want)
	}
}

func TestCacheKeyLLMUsageVersioned(t *testing.T) {
	got := cacheKeyLLMUsageDailySummaryVersioned("u1", 5, 14)
	wantParts := []string{
		"v1:llm_usage:daily:u1:v=5",
		"days=14",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("cacheKeyLLMUsageDailySummaryVersioned missing %q in %q", part, got)
		}
	}

	got = cacheKeyLLMUsageExecutionCurrentMonthVersioned("u1", 11, "2026-03")
	want := "v1:llm_usage:execution_current_month:u1:v=11:month=2026-03"
	if got != want {
		t.Fatalf("cacheKeyLLMUsageExecutionCurrentMonthVersioned = %q, want %q", got, want)
	}

	if got, want = cacheKeyLLMUsageModelSummaryVersioned("u1", 5, 30), "v1:llm_usage:model:u1:v=5:days=30"; got != want {
		t.Fatalf("cacheKeyLLMUsageModelSummaryVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsageProviderCurrentMonthVersioned("u1", 2, "2026-03"), "v1:llm_usage:provider_current_month:u1:v=2:month=2026-03"; got != want {
		t.Fatalf("cacheKeyLLMUsageProviderCurrentMonthVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsagePurposeCurrentMonthVersioned("u1", 4, "2026-03"), "v1:llm_usage:purpose_current_month:u1:v=4:month=2026-03"; got != want {
		t.Fatalf("cacheKeyLLMUsagePurposeCurrentMonthVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsageValueMetricsCurrentMonthVersioned("u1", 7, "2026-03"), "v1:llm_usage:value_metrics_current_month:u1:v=7:month=2026-03"; got != want {
		t.Fatalf("cacheKeyLLMUsageValueMetricsCurrentMonthVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsageListVersioned("u1", 6, 100), "v1:llm_usage:list:u1:v=6:limit=100"; got != want {
		t.Fatalf("cacheKeyLLMUsageListVersioned = %q, want %q", got, want)
	}
}

func TestCacheKeyTriageQueue(t *testing.T) {
	got := cacheKeyTriageQueue("u1", "24h", 15, true, true)
	want := "v1:items:triage-queue:u1:window=24h:size=15:div=true:exclude_later=true"
	if got != want {
		t.Fatalf("cacheKeyTriageQueue = %q, want %q", got, want)
	}
}
