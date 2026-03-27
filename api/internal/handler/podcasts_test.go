package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type stubPodcastFeedBuilder struct {
	body         []byte
	lastModified time.Time
	err          error
	slug         string
}

func (s *stubPodcastFeedBuilder) Build(_ context.Context, slug string) (*service.PodcastFeedResult, error) {
	s.slug = slug
	if s.err != nil {
		return nil, s.err
	}
	return &service.PodcastFeedResult{
		Body:         s.body,
		LastModified: s.lastModified,
	}, nil
}

func newPodcastsRequest(method string) *http.Request {
	req := httptest.NewRequest(method, "/podcasts/test-slug/feed.xml", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("slug", "test-slug")
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func TestPodcastsFeedHeadReturnsHeadersWithoutBody(t *testing.T) {
	lastModified := time.Date(2026, 3, 27, 12, 34, 56, 0, time.UTC)
	builder := &stubPodcastFeedBuilder{
		body:         []byte(`<rss version="2.0"></rss>`),
		lastModified: lastModified,
	}
	h := NewPodcastsHandler(builder)
	rec := httptest.NewRecorder()

	h.Feed(rec, newPodcastsRequest(http.MethodHead))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/rss+xml; charset=utf-8" {
		t.Fatalf("expected content type to be rss xml, got %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "25" {
		t.Fatalf("expected content length 25, got %q", got)
	}
	if got := rec.Header().Get("ETag"); got == "" {
		t.Fatal("expected etag header")
	}
	if got := rec.Header().Get("Last-Modified"); got != lastModified.Format(http.TimeFormat) {
		t.Fatalf("expected last modified %q, got %q", lastModified.Format(http.TimeFormat), got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected no body for HEAD, got %q", rec.Body.String())
	}
	if builder.slug != "test-slug" {
		t.Fatalf("expected slug test-slug, got %q", builder.slug)
	}
}

func TestPodcastsFeedGetReturnsBody(t *testing.T) {
	body := []byte(`<rss version="2.0"></rss>`)
	h := NewPodcastsHandler(&stubPodcastFeedBuilder{body: body})
	rec := httptest.NewRecorder()

	h.Feed(rec, newPodcastsRequest(http.MethodGet))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != string(body) {
		t.Fatalf("expected body %q, got %q", string(body), rec.Body.String())
	}
}

func TestPodcastsFeedNotFound(t *testing.T) {
	h := NewPodcastsHandler(&stubPodcastFeedBuilder{err: repository.ErrNotFound})
	rec := httptest.NewRecorder()

	h.Feed(rec, newPodcastsRequest(http.MethodGet))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestPodcastsFeedInternalError(t *testing.T) {
	h := NewPodcastsHandler(&stubPodcastFeedBuilder{err: errors.New("boom")})
	rec := httptest.NewRecorder()

	h.Feed(rec, newPodcastsRequest(http.MethodGet))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}
