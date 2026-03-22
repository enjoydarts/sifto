package service

import "testing"

func TestNewMeilisearchServiceFromEnv(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "http://meilisearch:7700")
	t.Setenv("MEILISEARCH_MASTER_KEY", "change-me")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "")

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
}

func TestNewMeilisearchServiceFromEnvCustomIndex(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "http://meilisearch:7700")
	t.Setenv("MEILISEARCH_MASTER_KEY", "change-me")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "items_dev")

	svc, err := NewMeilisearchServiceFromEnv()
	if err != nil {
		t.Fatalf("NewMeilisearchServiceFromEnv returned error: %v", err)
	}
	if got := svc.ItemsIndexName(); got != "items_dev" {
		t.Fatalf("ItemsIndexName = %q, want %q", got, "items_dev")
	}
}

func TestNewMeilisearchServiceFromEnvRequiresURL(t *testing.T) {
	t.Setenv("MEILISEARCH_URL", "")
	t.Setenv("MEILISEARCH_MASTER_KEY", "")
	t.Setenv("MEILISEARCH_ITEMS_INDEX", "")

	svc, err := NewMeilisearchServiceFromEnv()
	if err == nil {
		t.Fatal("NewMeilisearchServiceFromEnv error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("NewMeilisearchServiceFromEnv service = non-nil, want nil")
	}
}
