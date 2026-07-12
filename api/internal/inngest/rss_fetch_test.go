package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/mmcdole/gofeed"
)

func TestFetchRSSFeedUsesConditionalHeaders(t *testing.T) {
	etag := `"feed-v1"`
	lastModified := "Sat, 11 Jul 2026 00:00:00 GMT"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-None-Match"); got != etag {
			t.Fatalf("If-None-Match = %q, want %q", got, etag)
		}
		if got := r.Header.Get("If-Modified-Since"); got != lastModified {
			t.Fatalf("If-Modified-Since = %q, want %q", got, lastModified)
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	feed, notModified, gotETag, gotLastModified, err := fetchRSSFeed(context.Background(), server.Client(), model.Source{
		URL:              server.URL,
		FeedETag:         &etag,
		FeedLastModified: &lastModified,
	})
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if feed != nil || !notModified {
		t.Fatalf("feed = %#v, notModified = %v; want nil, true", feed, notModified)
	}
	if gotETag == nil || *gotETag != etag || gotLastModified == nil || *gotLastModified != lastModified {
		t.Fatalf("metadata = %v, %v; want %q, %q", gotETag, gotLastModified, etag, lastModified)
	}
}

func TestFetchRSSFeedParsesFeedAndCapturesMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"feed-v2"`)
		w.Header().Set("Last-Modified", "Sun, 12 Jul 2026 00:00:00 GMT")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title><item><link>https://example.com/1</link></item></channel></rss>`))
	}))
	defer server.Close()

	feed, notModified, etag, lastModified, err := fetchRSSFeed(context.Background(), server.Client(), model.Source{URL: server.URL})
	if err != nil {
		t.Fatalf("fetchRSSFeed() error = %v", err)
	}
	if notModified || feed == nil || len(feed.Items) != 1 {
		t.Fatalf("feed = %#v, notModified = %v; want one item, false", feed, notModified)
	}
	if etag == nil || *etag != `"feed-v2"` || lastModified == nil || *lastModified != "Sun, 12 Jul 2026 00:00:00 GMT" {
		t.Fatalf("unexpected metadata: %v, %v", etag, lastModified)
	}
}

func TestFeedItemURLsDeduplicatesAndSkipsEmptyLinks(t *testing.T) {
	feed := &gofeed.Feed{Items: []*gofeed.Item{
		{Link: "https://example.com/1"},
		{Link: "https://example.com/1"},
		{Link: " "},
		{Link: "https://example.com/2"},
	}}
	urls := feedItemURLs(feed)
	if len(urls) != 2 || urls[0] != "https://example.com/1" || urls[1] != "https://example.com/2" {
		t.Fatalf("feedItemURLs() = %#v", urls)
	}
}
