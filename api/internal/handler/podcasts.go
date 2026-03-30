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

func NewPodcastsHandler(feed podcastFeedBuilder, cache service.JSONCache) *PodcastsHandler {
	return &PodcastsHandler{feed: feed, cache: cache}
}

func (h *PodcastsHandler) Feed(w http.ResponseWriter, r *http.Request) {
	if h.feed == nil {
		http.Error(w, "podcast feed unavailable", http.StatusInternalServerError)
		return
	}
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	var result *service.PodcastFeedResult
	cacheKey := cacheKeyPodcastFeed(slug)
	if h.cache != nil && slug != "" {
		var cached service.PodcastFeedResult
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			result = &cached
		}
	}
	if result == nil {
		built, err := h.feed.Build(r.Context(), slug)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to build podcast feed", http.StatusInternalServerError)
			return
		}
		result = built
		if h.cache != nil && slug != "" {
			_ = h.cache.SetJSON(r.Context(), cacheKey, result, podcastFeedCacheTTL)
		}
	}
	body := result.Body
	sum := sha256.Sum256(body)
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("ETag", fmt.Sprintf(`W/"%x"`, sum))
	if !result.LastModified.IsZero() {
		w.Header().Set("Last-Modified", result.LastModified.UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

func cacheKeyPodcastFeed(slug string) string {
	return "v1:podcast:feed:slug=" + strings.TrimSpace(slug)
}
