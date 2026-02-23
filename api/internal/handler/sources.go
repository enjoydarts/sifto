package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mmcdole/gofeed"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
)

type FeedCandidate struct {
	URL   string  `json:"url"`
	Title *string `json:"title"`
}

var (
	reFeedLink1 = regexp.MustCompile(`(?i)<link[^>]+type="application/(rss|atom)\+xml"[^>]+href="([^"]+)"`)
	reFeedLink2 = regexp.MustCompile(`(?i)<link[^>]+href="([^"]+)"[^>]+type="application/(rss|atom)\+xml"`)
	reTitleAttr = regexp.MustCompile(`(?i)\btitle="([^"]+)"`)
)

func discoverRSSFeeds(ctx context.Context, rawURL string) ([]FeedCandidate, error) {
	// Step 1: Try parsing the URL directly as a feed.
	fp := gofeed.NewParser()
	if feed, err := fp.ParseURLWithContext(rawURL, ctx); err == nil {
		var t *string
		if feed.Title != "" {
			t = &feed.Title
		}
		return []FeedCandidate{{URL: rawURL, Title: t}}, nil
	}

	// Step 2: Fetch the URL as HTML and look for RSS/Atom <link> tags.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Sifto/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var candidates []FeedCandidate

	addCandidate := func(href string, tag []byte) {
		ref, e := url.Parse(href)
		if e != nil {
			return
		}
		absURL := base.ResolveReference(ref).String()
		if seen[absURL] {
			return
		}
		seen[absURL] = true
		var title *string
		if m := reTitleAttr.FindSubmatch(tag); m != nil {
			t := string(m[1])
			title = &t
		}
		candidates = append(candidates, FeedCandidate{URL: absURL, Title: title})
	}

	for _, m := range reFeedLink1.FindAllSubmatch(body, -1) {
		addCandidate(string(m[2]), m[0])
	}
	for _, m := range reFeedLink2.FindAllSubmatch(body, -1) {
		addCandidate(string(m[1]), m[0])
	}

	if len(candidates) == 0 {
		return nil, errors.New("指定されたURLからRSSフィードが見つかりませんでした")
	}
	return candidates, nil
}

type SourceHandler struct {
	repo      *repository.SourceRepo
	itemRepo  *repository.ItemRepo
	publisher *service.EventPublisher
}

func NewSourceHandler(repo *repository.SourceRepo, itemRepo *repository.ItemRepo, publisher *service.EventPublisher) *SourceHandler {
	return &SourceHandler{
		repo:      repo,
		itemRepo:  itemRepo,
		publisher: publisher,
	}
}

func (h *SourceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	sources, err := h.repo.List(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, sources)
}

func (h *SourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		URL   string  `json:"url"`
		Type  string  `json:"type"`
		Title *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" || body.Type == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	body.Type = strings.TrimSpace(body.Type)
	if body.URL == "" || body.Type == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	switch strings.ToLower(body.Type) {
	case "rss", "manual":
		body.Type = strings.ToLower(body.Type)
	default:
		http.Error(w, "invalid source type", http.StatusBadRequest)
		return
	}
	parsed, err := url.ParseRequestURI(body.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}

	s, err := h.repo.Create(r.Context(), userID, body.URL, body.Type, body.Title)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	// For one-off URLs, seed an item immediately and trigger async processing.
	if strings.EqualFold(body.Type, "manual") && h.itemRepo != nil {
		itemID, created, err := h.itemRepo.UpsertFromFeed(r.Context(), s.ID, body.URL, body.Title)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if created {
			h.publisher.SendItemCreated(r.Context(), itemID, s.ID, body.URL)
		}
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, s)
}

func (h *SourceHandler) Discover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	feeds, err := discoverRSSFeeds(r.Context(), strings.TrimSpace(body.URL))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	writeJSON(w, map[string]any{"feeds": feeds})
}

func (h *SourceHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	s, err := h.repo.Update(r.Context(), id, userID, *body.Enabled)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, s)
}

func (h *SourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id, userID); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
