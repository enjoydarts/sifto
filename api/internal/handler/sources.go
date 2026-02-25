package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/mmcdole/gofeed"
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
	repo         *repository.SourceRepo
	itemRepo     *repository.ItemRepo
	settingsRepo *repository.UserSettingsRepo
	llmUsageRepo *repository.LLMUsageLogRepo
	worker       *service.WorkerClient
	cipher       *service.SecretCipher
	publisher    *service.EventPublisher
}

func NewSourceHandler(
	repo *repository.SourceRepo,
	itemRepo *repository.ItemRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	worker *service.WorkerClient,
	cipher *service.SecretCipher,
	publisher *service.EventPublisher,
) *SourceHandler {
	return &SourceHandler{
		repo:         repo,
		itemRepo:     itemRepo,
		settingsRepo: settingsRepo,
		llmUsageRepo: llmUsageRepo,
		worker:       worker,
		cipher:       cipher,
		publisher:    publisher,
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

type sourceSuggestionResponse struct {
	URL           string   `json:"url"`
	Title         *string  `json:"title"`
	Reasons       []string `json:"reasons"`
	MatchedTopics []string `json:"matched_topics,omitempty"`
	AIReason      *string  `json:"ai_reason,omitempty"`
	AIConfidence  *float64 `json:"ai_confidence,omitempty"`
	SeedSourceIDs []string `json:"seed_source_ids"`
}

type sourceSuggestionAgg struct {
	URL           string
	Title         *string
	Reasons       map[string]bool
	MatchedTopics map[string]bool
	SeedSourceIDs map[string]bool
	Score         int
}

func (h *SourceHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	limit := parseIntOrDefault(q.Get("limit"), 10)
	if limit < 1 || limit > 30 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}

	sources, err := h.repo.List(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if len(sources) == 0 {
		writeJSON(w, map[string]any{"items": []sourceSuggestionResponse{}, "limit": limit})
		return
	}
	anthropicAPIKey := h.getUserAnthropicAPIKey(r.Context(), userID)
	var preferredTopics []string
	if h.itemRepo != nil {
		if topics, err := h.itemRepo.PositiveFeedbackTopics(r.Context(), userID, 8); err == nil {
			preferredTopics = topics
		}
	}

	registered := map[string]bool{}
	type probeSeed struct {
		SourceID string
		ProbeURL string
		Reason   string
	}
	var probes []probeSeed
	seenProbe := map[string]bool{}
	for _, s := range sources {
		registered[normalizeFeedURL(s.URL)] = true
		for _, p := range suggestionProbeURLs(s.URL) {
			if seenProbe[p.url] {
				continue
			}
			seenProbe[p.url] = true
			probes = append(probes, probeSeed{
				SourceID: s.ID,
				ProbeURL: p.url,
				Reason:   p.reason,
			})
		}
	}

	// Keep response latency predictable.
	if len(probes) > 16 {
		probes = probes[:16]
	}

	cands := map[string]*sourceSuggestionAgg{}
	for _, p := range probes {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		feeds, err := discoverRSSFeeds(ctx, p.ProbeURL)
		cancel()
		if err != nil {
			continue
		}
		for _, f := range feeds {
			key := normalizeFeedURL(f.URL)
			if key == "" || registered[key] {
				continue
			}
			a := cands[key]
			if a == nil {
				a = &sourceSuggestionAgg{
					URL:           f.URL,
					Title:         f.Title,
					Reasons:       map[string]bool{},
					MatchedTopics: map[string]bool{},
					SeedSourceIDs: map[string]bool{},
				}
				cands[key] = a
			}
			if a.Title == nil && f.Title != nil {
				a.Title = f.Title
			}
			if !a.Reasons[p.Reason] {
				a.Reasons[p.Reason] = true
				a.Score++
			}
			if !a.SeedSourceIDs[p.SourceID] {
				a.SeedSourceIDs[p.SourceID] = true
				a.Score += 2
			}
			for _, topic := range preferredTopics {
				if topic == "" {
					continue
				}
				if sourceSuggestionTopicMatch(f, topic) {
					if !a.MatchedTopics[topic] {
						a.MatchedTopics[topic] = true
						a.Score += 3
					}
				}
			}
		}
	}
	if len(cands) < minInt(limit, 4) && anthropicAPIKey != nil && h.worker != nil {
		h.expandSourceSuggestionsWithLLMSeeds(r.Context(), userID, sources, preferredTopics, registered, cands, anthropicAPIKey)
	}

	out := make([]sourceSuggestionResponse, 0, len(cands))
	type sortable struct {
		row   sourceSuggestionResponse
		score int
	}
	var rows []sortable
	for _, a := range cands {
		reasons := mapKeys(a.Reasons)
		matchedTopics := mapKeys(a.MatchedTopics)
		seedIDs := mapKeys(a.SeedSourceIDs)
		if len(matchedTopics) > 0 {
			reasons = append([]string{"高評価トピックに近い候補"}, reasons...)
		}
		rows = append(rows, sortable{
			score: a.Score,
			row: sourceSuggestionResponse{
				URL:           a.URL,
				Title:         a.Title,
				Reasons:       reasons,
				MatchedTopics: matchedTopics,
				SeedSourceIDs: seedIDs,
			},
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if rows[i].row.Title != nil && rows[j].row.Title != nil && *rows[i].row.Title != *rows[j].row.Title {
			return *rows[i].row.Title < *rows[j].row.Title
		}
		return rows[i].row.URL < rows[j].row.URL
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	for _, r := range rows {
		out = append(out, r.row)
	}
	llmMeta := h.rankSourceSuggestionsWithLLM(r.Context(), userID, sources, preferredTopics, out, anthropicAPIKey)
	writeJSON(w, map[string]any{"items": out, "limit": limit, "llm": llmMeta})
}

type suggestionProbe struct {
	url    string
	reason string
}

func suggestionProbeURLs(raw string) []suggestionProbe {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []suggestionProbe
	add := func(v, reason string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, suggestionProbe{url: v, reason: reason})
	}
	root := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/"}
	add(root.String(), "同一サイトのトップページから発見")

	cleanPath := path.Clean(u.Path)
	if cleanPath != "." && cleanPath != "/" {
		parent := path.Dir(cleanPath)
		if parent != "." && parent != "/" {
			parentURL := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: parent + "/"}
			add(parentURL.String(), "登録ソースの親URLから発見")
		}
	}
	return out
}

func normalizeFeedURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	u.Fragment = ""
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) || (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = strings.Split(u.Host, ":")[0]
	}
	u.Host = strings.ToLower(u.Host)
	return u.String()
}

func mapKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sourceSuggestionTopicMatch(f FeedCandidate, topic string) bool {
	t := strings.ToLower(strings.TrimSpace(topic))
	if t == "" {
		return false
	}
	if f.Title != nil && strings.Contains(strings.ToLower(*f.Title), t) {
		return true
	}
	return strings.Contains(strings.ToLower(f.URL), t)
}

func (h *SourceHandler) rankSourceSuggestionsWithLLM(
	ctx context.Context,
	userID string,
	sources []model.Source,
	preferredTopics []string,
	suggestions []sourceSuggestionResponse,
	anthropicAPIKey *string,
) map[string]any {
	if h.worker == nil || len(suggestions) == 0 || anthropicAPIKey == nil || strings.TrimSpace(*anthropicAPIKey) == "" {
		return nil
	}
	existing := make([]service.RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, s := range sources {
		existing = append(existing, service.RankFeedSuggestionsExistingSource{
			URL:   s.URL,
			Title: s.Title,
		})
	}
	cands := make([]service.RankFeedSuggestionsCandidate, 0, len(suggestions))
	for _, s := range suggestions {
		cands = append(cands, service.RankFeedSuggestionsCandidate{
			URL:           s.URL,
			Title:         s.Title,
			Reasons:       s.Reasons,
			MatchedTopics: s.MatchedTopics,
		})
	}
	resp, err := h.worker.RankFeedSuggestions(ctx, existing, preferredTopics, cands, anthropicAPIKey)
	if err != nil || resp == nil {
		return nil
	}
	h.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	if len(resp.Items) == 0 {
		if resp.LLM == nil {
			return nil
		}
		return map[string]any{
			"provider":             resp.LLM.Provider,
			"model":                resp.LLM.Model,
			"estimated_cost_usd":   resp.LLM.EstimatedCostUSD,
			"input_tokens":         resp.LLM.InputTokens,
			"output_tokens":        resp.LLM.OutputTokens,
			"pricing_source":       resp.LLM.PricingSource,
			"pricing_model_family": resp.LLM.PricingModelFamily,
		}
	}
	byURL := map[string]*sourceSuggestionResponse{}
	for i := range suggestions {
		byURL[normalizeFeedURL(suggestions[i].URL)] = &suggestions[i]
	}
	rank := make(map[string]int, len(resp.Items))
	for i, it := range resp.Items {
		k := normalizeFeedURL(it.URL)
		if k == "" {
			continue
		}
		rank[k] = i
		if s, ok := byURL[k]; ok {
			reason := strings.TrimSpace(it.Reason)
			if reason != "" {
				s.AIReason = &reason
			}
			conf := it.Confidence
			s.AIConfidence = &conf
		}
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		ki := normalizeFeedURL(suggestions[i].URL)
		kj := normalizeFeedURL(suggestions[j].URL)
		ri, iok := rank[ki]
		rj, jok := rank[kj]
		if iok && jok && ri != rj {
			return ri < rj
		}
		if iok != jok {
			return iok
		}
		return suggestions[i].URL < suggestions[j].URL
	})
	if resp.LLM == nil {
		return nil
	}
	return map[string]any{
		"provider":             resp.LLM.Provider,
		"model":                resp.LLM.Model,
		"estimated_cost_usd":   resp.LLM.EstimatedCostUSD,
		"input_tokens":         resp.LLM.InputTokens,
		"output_tokens":        resp.LLM.OutputTokens,
		"pricing_source":       resp.LLM.PricingSource,
		"pricing_model_family": resp.LLM.PricingModelFamily,
	}
}

func (h *SourceHandler) getUserAnthropicAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetAnthropicAPIKeyEncrypted(ctx, userID)
	if err != nil || enc == nil || *enc == "" {
		return nil
	}
	plain, err := h.cipher.DecryptString(*enc)
	if err != nil {
		return nil
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil
	}
	return &plain
}

func (h *SourceHandler) expandSourceSuggestionsWithLLMSeeds(
	ctx context.Context,
	userID string,
	sources []model.Source,
	preferredTopics []string,
	registered map[string]bool,
	cands map[string]*sourceSuggestionAgg,
	anthropicAPIKey *string,
) {
	existing := make([]service.RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, s := range sources {
		existing = append(existing, service.RankFeedSuggestionsExistingSource{URL: s.URL, Title: s.Title})
	}
	resp, err := h.worker.SuggestFeedSeedSites(ctx, existing, preferredTopics, anthropicAPIKey)
	if err != nil || resp == nil {
		return
	}
	h.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	for _, seed := range resp.Items {
		ctxOne, cancel := context.WithTimeout(ctx, 8*time.Second)
		feeds, err := discoverRSSFeeds(ctxOne, strings.TrimSpace(seed.URL))
		cancel()
		if err != nil {
			continue
		}
		for _, f := range feeds {
			key := normalizeFeedURL(f.URL)
			if key == "" || registered[key] {
				continue
			}
			a := cands[key]
			if a == nil {
				a = &sourceSuggestionAgg{
					URL:           f.URL,
					Title:         f.Title,
					Reasons:       map[string]bool{},
					MatchedTopics: map[string]bool{},
					SeedSourceIDs: map[string]bool{},
				}
				cands[key] = a
			}
			if a.Title == nil && f.Title != nil {
				a.Title = f.Title
			}
			reason := "AI提案サイトから発見"
			if strings.TrimSpace(seed.Reason) != "" {
				reason = "AI候補: " + strings.TrimSpace(seed.Reason)
			}
			if !a.Reasons[reason] {
				a.Reasons[reason] = true
				a.Score += 2
			}
			for _, topic := range preferredTopics {
				if sourceSuggestionTopicMatch(f, topic) && !a.MatchedTopics[topic] {
					a.MatchedTopics[topic] = true
					a.Score += 3
				}
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *SourceHandler) recordSourceSuggestionLLMUsage(ctx context.Context, userID string, llm *service.LLMUsage) {
	if h.llmUsageRepo == nil || llm == nil {
		return
	}
	if llm.Provider == "" || llm.Model == "" {
		return
	}
	uid := userID
	if err := h.llmUsageRepo.Insert(ctx, repository.LLMUsageLogInput{
		UserID:                   &uid,
		Provider:                 llm.Provider,
		Model:                    llm.Model,
		PricingModelFamily:       llm.PricingModelFamily,
		PricingSource:            llm.PricingSource,
		Purpose:                  "source_suggestion",
		InputTokens:              llm.InputTokens,
		OutputTokens:             llm.OutputTokens,
		CacheCreationInputTokens: llm.CacheCreationInputTokens,
		CacheReadInputTokens:     llm.CacheReadInputTokens,
		EstimatedCostUSD:         llm.EstimatedCostUSD,
	}); err != nil {
		// Best-effort logging: don't fail source suggestions UI on usage log issues.
	}
}

func (h *SourceHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled *bool   `json:"enabled"`
		Title   *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Enabled == nil && body.Title == nil) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var title *string
	updateTitle := body.Title != nil
	if body.Title != nil {
		v := strings.TrimSpace(*body.Title)
		if v != "" {
			title = &v
		}
	}
	s, err := h.repo.Update(r.Context(), id, userID, body.Enabled, updateTitle, title)
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
