package handler

import (
	"errors"
	"net/http"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type PodcastsHandler struct {
	feed *service.PodcastFeedService
}

func NewPodcastsHandler(feed *service.PodcastFeedService) *PodcastsHandler {
	return &PodcastsHandler{feed: feed}
}

func (h *PodcastsHandler) Feed(w http.ResponseWriter, r *http.Request) {
	if h.feed == nil {
		http.Error(w, "podcast feed unavailable", http.StatusInternalServerError)
		return
	}
	body, err := h.feed.BuildXML(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to build podcast feed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
