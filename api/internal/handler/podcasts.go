package handler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type podcastFeedBuilder interface {
	Build(ctx context.Context, slug string) (*service.PodcastFeedResult, error)
}

type PodcastsHandler struct {
	feed podcastFeedBuilder
}

func NewPodcastsHandler(feed podcastFeedBuilder) *PodcastsHandler {
	return &PodcastsHandler{feed: feed}
}

func (h *PodcastsHandler) Feed(w http.ResponseWriter, r *http.Request) {
	if h.feed == nil {
		http.Error(w, "podcast feed unavailable", http.StatusInternalServerError)
		return
	}
	result, err := h.feed.Build(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to build podcast feed", http.StatusInternalServerError)
		return
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
