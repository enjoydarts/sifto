package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeedDiscoveryDoesNotConnectToPrivateHost(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write([]byte("<rss version=\"2.0\"><channel><title>Private</title></channel></rss>"))
	}))
	defer server.Close()
	if _, err := DiscoverRSSFeeds(context.Background(), server.URL); err == nil {
		t.Fatal("private feed accepted")
	}
	if requests != 0 {
		t.Fatalf("made %d private requests", requests)
	}
}
