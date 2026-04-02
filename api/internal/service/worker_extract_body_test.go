package service

import "testing"

func TestExtractWorkerErrorDetailPrefersMessageField(t *testing.T) {
	body := []byte(`{"detail":{"code":"youtube_transcript_unavailable","message":"youtube transcript unavailable: manual_langs=[] auto_langs=['en-US'] auto_exts=['srv3']","title":"Video Title"}}`)

	got := extractWorkerErrorDetail(body)
	want := "youtube transcript unavailable: manual_langs=[] auto_langs=['en-US'] auto_exts=['srv3']"
	if got != want {
		t.Fatalf("extractWorkerErrorDetail() = %q, want %q", got, want)
	}
}

func TestExtractBodyPartialFromError(t *testing.T) {
	body := []byte(`{"detail":{"code":"youtube_transcript_unavailable","message":"youtube transcript unavailable","title":"Video Title","published_at":"2026-04-02","image_url":"https://img.example/thumb.jpg"}}`)

	got := extractBodyPartialFromError(body)
	if got == nil {
		t.Fatal("extractBodyPartialFromError() = nil")
	}
	if got.Title == nil || *got.Title != "Video Title" {
		t.Fatalf("title = %#v, want Video Title", got.Title)
	}
	if got.PublishedAt == nil || *got.PublishedAt != "2026-04-02" {
		t.Fatalf("published_at = %#v, want 2026-04-02", got.PublishedAt)
	}
	if got.ImageURL == nil || *got.ImageURL != "https://img.example/thumb.jpg" {
		t.Fatalf("image_url = %#v, want thumb url", got.ImageURL)
	}
}
