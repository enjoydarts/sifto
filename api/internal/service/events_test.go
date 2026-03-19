package service

import "testing"

func TestNewItemCreatedEventIncludesReasonAndTriggerID(t *testing.T) {
	title := "Example title"

	event := NewItemCreatedEvent("item-1", "source-1", "https://example.com/a", &title, "retry")

	if event.Name != "item/created" {
		t.Fatalf("event.Name = %q, want %q", event.Name, "item/created")
	}
	data := event.Data
	if got := data["item_id"]; got != "item-1" {
		t.Fatalf("item_id = %v, want %q", got, "item-1")
	}
	if got := data["source_id"]; got != "source-1" {
		t.Fatalf("source_id = %v, want %q", got, "source-1")
	}
	if got := data["url"]; got != "https://example.com/a" {
		t.Fatalf("url = %v, want %q", got, "https://example.com/a")
	}
	if got := data["title"]; got != title {
		t.Fatalf("title = %v, want %q", got, title)
	}
	if got := data["reason"]; got != "retry" {
		t.Fatalf("reason = %v, want %q", got, "retry")
	}
	triggerID, _ := data["trigger_id"].(string)
	if triggerID == "" {
		t.Fatalf("trigger_id = %q, want non-empty", triggerID)
	}
}
