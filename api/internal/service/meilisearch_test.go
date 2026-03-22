package service

import "testing"

func TestNewMeilisearchServiceFromEnv(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "http://meilisearch:7700")
	t.Setenv("MEILISEARCH_MASTER_KEY", "change-me")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "")
	t.Setenv("MEILISEARCH_SUGGESTIONS_INDEX", "")

	svc, err := NewMeilisearchServiceFromEnv()
	if err != nil {
		t.Fatalf("NewMeilisearchServiceFromEnv returned error: %v", err)
	}
	if svc == nil {
		t.Fatal("NewMeilisearchServiceFromEnv returned nil service")
	}
	if got := svc.ItemsIndexName(); got != "items" {
		t.Fatalf("ItemsIndexName = %q, want %q", got, "items")
	}
	if got := svc.SuggestionsIndexName(); got != "search_suggestions" {
		t.Fatalf("SuggestionsIndexName = %q, want %q", got, "search_suggestions")
	}
}

func TestNewMeilisearchServiceFromEnvCustomIndex(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "http://meilisearch:7700")
	t.Setenv("MEILISEARCH_MASTER_KEY", "change-me")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "items_dev")
	t.Setenv("MEILISEARCH_SUGGESTIONS_INDEX", "search_suggestions_dev")

	svc, err := NewMeilisearchServiceFromEnv()
	if err != nil {
		t.Fatalf("NewMeilisearchServiceFromEnv returned error: %v", err)
	}
	if got := svc.ItemsIndexName(); got != "items_dev" {
		t.Fatalf("ItemsIndexName = %q, want %q", got, "items_dev")
	}
	if got := svc.SuggestionsIndexName(); got != "search_suggestions_dev" {
		t.Fatalf("SuggestionsIndexName = %q, want %q", got, "search_suggestions_dev")
	}
}

func TestNewMeilisearchServiceFromEnvRequiresURL(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "")
	t.Setenv("MEILISEARCH_MASTER_KEY", "")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "")
	t.Setenv("MEILISEARCH_SUGGESTIONS_INDEX", "")

	svc, err := NewMeilisearchServiceFromEnv()
	if err == nil {
		t.Fatal("NewMeilisearchServiceFromEnv error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("NewMeilisearchServiceFromEnv service = non-nil, want nil")
	}
}
