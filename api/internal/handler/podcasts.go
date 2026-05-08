package handler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type podcastFeedBuilder interface {
	Build(ctx context.Context, slug string) (*service.PodcastFeedResult, error)
}

type PodcastsHandler struct {
	feed  podcastFeedBuilder
	cache service.JSONCache
}

const podcastFeedCacheTTL = 10 * time.Minute
const podcastFeedCacheControl = "public, max-age=300, s-maxage=600, stale-while-revalidate=1800"

func NewPodcastsHandler(feed podcastFeedBuilder, cache service.JSONCache) *PodcastsHandler {
	return &PodcastsHandler{feed: feed, cache: cache}
}

func (h *PodcastsHandler) Feed(w http.ResponseWriter, r *http.Request) {
	if h.feed == nil {
		http.Error(w, "podcast feed unavailable", http.StatusInternalServerError)
		return
	}
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	cacheKey := cacheKeyPodcastFeed(slug)
	result, err := cachedFetch(r.Context(), h.cache, cacheKey, podcastFeedCacheTTL, func() (*service.PodcastFeedResult, error) {
		return h.feed.Build(r.Context(), slug)
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to build podcast feed", http.StatusInternalServerError)
		return
	}
	if result == nil {
		http.Error(w, "failed to build podcast feed", http.StatusInternalServerError)
		return
	}
	body := result.Body
	sum := sha256.Sum256(body)
	etag := fmt.Sprintf(`W/"%x"`, sum)
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", podcastFeedCacheControl)
	w.Header().Set("ETag", etag)
	if !result.LastModified.IsZero() {
		w.Header().Set("Last-Modified", result.LastModified.UTC().Format(http.TimeFormat))
	}
	if podcastFeedNotModified(r, etag, result.LastModified) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

func podcastFeedNotModified(r *http.Request, etag string, lastModified time.Time) bool {
	if r == nil {
		return false
	}
	if inm := strings.TrimSpace(r.Header.Get("If-None-Match")); inm != "" {
		for _, candidate := range strings.Split(inm, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "*" || candidate == etag {
				return true
			}
		}
		return false
	}
	if lastModified.IsZero() {
		return false
	}
	raw := strings.TrimSpace(r.Header.Get("If-Modified-Since"))
	if raw == "" {
		return false
	}
	since, err := http.ParseTime(raw)
	if err != nil {
		return false
	}
	return !lastModified.UTC().After(since.UTC())
}

func cacheKeyPodcastFeed(slug string) string {
	return "v1:podcast:feed:slug=" + strings.TrimSpace(slug)
}
