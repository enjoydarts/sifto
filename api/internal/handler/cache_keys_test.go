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
	if got, want := cacheVersionKeyUserLLMUsage("u1"), "cache_version:user_llm_usage:u1"; got != want {
		t.Fatalf("cacheVersionKeyUserLLMUsage = %q, want %q", got, want)
	}
}

func TestCacheKeyItemsListVersioned(t *testing.T) {
	got := cacheKeyItemsListVersioned("u1", 7, "summarized", "src-1", "go", "openai", true, false, true, false, "score", 2, 50)
	wantParts := []string{
		"v1:items:list:u1:v=7",
		"status=summarized",
		"source=src-1",
		"topic=go",
		"q=openai",
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

	got = cacheKeyLLMUsageExecutionCurrentMonthVersioned("u1", 11)
	want := "v1:llm_usage:execution_current_month:u1:v=11"
	if got != want {
		t.Fatalf("cacheKeyLLMUsageExecutionCurrentMonthVersioned = %q, want %q", got, want)
	}

	if got, want = cacheKeyLLMUsageModelSummaryVersioned("u1", 5, 30), "v1:llm_usage:model:u1:v=5:days=30"; got != want {
		t.Fatalf("cacheKeyLLMUsageModelSummaryVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsageProviderCurrentMonthVersioned("u1", 2), "v1:llm_usage:provider_current_month:u1:v=2"; got != want {
		t.Fatalf("cacheKeyLLMUsageProviderCurrentMonthVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsagePurposeCurrentMonthVersioned("u1", 4), "v1:llm_usage:purpose_current_month:u1:v=4"; got != want {
		t.Fatalf("cacheKeyLLMUsagePurposeCurrentMonthVersioned = %q, want %q", got, want)
	}
	if got, want = cacheKeyLLMUsageListVersioned("u1", 6, 100), "v1:llm_usage:list:u1:v=6:limit=100"; got != want {
		t.Fatalf("cacheKeyLLMUsageListVersioned = %q, want %q", got, want)
	}
}
