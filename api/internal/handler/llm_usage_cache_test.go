package handler

import (
	"context"
	"testing"
)

func TestLLMUsageCacheKeysUseVersion(t *testing.T) {
	cache := newTestJSONCache()
	cache.versions[cacheVersionKeyUserLLMUsage("u1")] = 4
	handler := &LLMUsageHandler{cache: cache}

	key, err := handler.llmUsageCacheKey(context.Background(), "u1", cacheKeyLLMUsageDailySummaryVersioned("u1", 4, 14))
	if err != nil {
		t.Fatalf("llmUsageCacheKey returned error: %v", err)
	}
	want := "v1:llm_usage:daily:u1:v=4:days=14"
	if key != want {
		t.Fatalf("llmUsageCacheKey = %q, want %q", key, want)
	}
}

func TestBumpUserLLMUsageVersion(t *testing.T) {
	cache := newTestJSONCache()
	handler := &LLMUsageHandler{cache: cache}

	if err := handler.bumpUserLLMUsageVersion(context.Background(), "u1"); err != nil {
		t.Fatalf("first bumpUserLLMUsageVersion returned error: %v", err)
	}
	if err := handler.bumpUserLLMUsageVersion(context.Background(), "u1"); err != nil {
		t.Fatalf("second bumpUserLLMUsageVersion returned error: %v", err)
	}

	if got, want := cache.versions[cacheVersionKeyUserLLMUsage("u1")], int64(2); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
}

func TestLLMUsageExecutionSummaryCacheKeyUsesVersionAndDays(t *testing.T) {
	cache := newTestJSONCache()
	cache.versions[cacheVersionKeyUserLLMUsage("u1")] = 6
	handler := &LLMUsageHandler{cache: cache}

	key, err := handler.llmUsageCacheKey(context.Background(), "u1", cacheKeyLLMUsageExecutionSummaryVersioned("u1", 6, 30))
	if err != nil {
		t.Fatalf("llmUsageCacheKey returned error: %v", err)
	}
	want := "v1:llm_usage:execution:u1:v=6:days=30"
	if key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
}
