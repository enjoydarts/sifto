package handler

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
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

type opmlDocument struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title       string `xml:"title,omitempty"`
	DateCreated string `xml:"dateCreated,omitempty"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline,omitempty"`
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

func (h *SourceHandler) ExportOPML(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	sources, err := h.repo.List(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	outlines := make([]opmlOutline, 0, len(sources))
	for _, s := range sources {
		label := strings.TrimSpace(s.URL)
		if s.Title != nil && strings.TrimSpace(*s.Title) != "" {
			label = strings.TrimSpace(*s.Title)
		}
		outlines = append(outlines, opmlOutline{
			Text:    label,
			Title:   label,
			Type:    "rss",
			XMLURL:  s.URL,
			HTMLURL: s.URL,
		})
	}
	doc := opmlDocument{
		Version: "2.0",
		Head: opmlHead{
			Title:       "Sifto Sources Export",
			DateCreated: time.Now().UTC().Format(time.RFC1123Z),
		},
		Body: opmlBody{
			Outlines: outlines,
		},
	}
	payload, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, "failed to export opml", http.StatusInternalServerError)
		return
	}
	filename := fmt.Sprintf("sifto-sources-%s.opml", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(payload)
}

func (h *SourceHandler) ImportOPML(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		OPML string `json:"opml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.OPML) == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var doc opmlDocument
	if err := xml.Unmarshal([]byte(body.OPML), &doc); err != nil {
		http.Error(w, "invalid opml", http.StatusBadRequest)
		return
	}
	urlTitlePairs := flattenOPMLOutlines(doc.Body.Outlines)
	writeJSON(w, importURLTitlePairs(r.Context(), h.repo, userID, urlTitlePairs))
}

func (h *SourceHandler) ImportInoreader(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
	}
	token := strings.TrimSpace(body.AccessToken)
	if token == "" && h.settingsRepo != nil && h.cipher != nil && h.cipher.Enabled() {
		enc, _, _, err := h.settingsRepo.GetInoreaderTokensEncrypted(r.Context(), userID)
		if err == nil && enc != nil && strings.TrimSpace(*enc) != "" {
			if dec, decErr := h.cipher.DecryptString(*enc); decErr == nil {
				token = strings.TrimSpace(dec)
			}
		}
	}
	if token == "" {
		http.Error(w, "inoreader access token is not configured", http.StatusBadRequest)
		return
	}
	pairs, err := fetchInoreaderSubscriptions(r.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, importURLTitlePairs(r.Context(), h.repo, userID, pairs))
}

func (h *SourceHandler) Health(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.repo.HealthByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"items": rows,
	})
}

type opmlURLTitle struct {
	URL   string
	Title *string
}

type inoreaderSubscriptionListResponse struct {
	Subscriptions []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		HTMLURL string `json:"htmlUrl"`
	} `json:"subscriptions"`
}

func flattenOPMLOutlines(outlines []opmlOutline) []opmlURLTitle {
	out := make([]opmlURLTitle, 0)
	var walk func(rows []opmlOutline)
	walk = func(rows []opmlOutline) {
		for _, o := range rows {
			if strings.TrimSpace(o.XMLURL) != "" {
				var title *string
				if strings.TrimSpace(o.Title) != "" {
					t := strings.TrimSpace(o.Title)
					title = &t
				} else if strings.TrimSpace(o.Text) != "" {
					t := strings.TrimSpace(o.Text)
					title = &t
				}
				out = append(out, opmlURLTitle{
					URL:   strings.TrimSpace(o.XMLURL),
					Title: title,
				})
			}
			if len(o.Outlines) > 0 {
				walk(o.Outlines)
			}
		}
	}
	walk(outlines)
	return out
}

func importURLTitlePairs(ctx context.Context, repo *repository.SourceRepo, userID string, pairs []opmlURLTitle) map[string]any {
	added := 0
	skipped := 0
	invalid := 0
	errorsOut := make([]string, 0)
	for _, pair := range pairs {
		u := strings.TrimSpace(pair.URL)
		parsed, err := url.ParseRequestURI(u)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			invalid++
			continue
		}
		title := pair.Title
		if title != nil {
			v := strings.TrimSpace(*title)
			if v == "" {
				title = nil
			} else {
				title = &v
			}
		}
		if _, err := repo.Create(ctx, userID, u, "rss", title); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				skipped++
				continue
			}
			errorsOut = append(errorsOut, err.Error())
			if len(errorsOut) >= 10 {
				break
			}
			continue
		}
		added++
	}
	return map[string]any{
		"status":       "ok",
		"total":        len(pairs),
		"added":        added,
		"skipped":      skipped,
		"invalid":      invalid,
		"error_count":  len(errorsOut),
		"error_sample": errorsOut,
	}
}

func fetchInoreaderSubscriptions(ctx context.Context, accessToken string) ([]opmlURLTitle, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("access token is required")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	endpoint := "https://www.inoreader.com/reader/api/0/subscription/list?output=json"
	call := func(authHeader string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Sifto/1.0")
		return client.Do(req)
	}

	resp, err := call("GoogleLogin auth=" + accessToken)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		resp, err = call("Bearer " + accessToken)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("inoreader api status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded inoreaderSubscriptionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode inoreader subscriptions: %w", err)
	}
	out := make([]opmlURLTitle, 0, len(decoded.Subscriptions))
	seen := map[string]struct{}{}
	for _, s := range decoded.Subscriptions {
		raw := strings.TrimSpace(s.ID)
		if strings.HasPrefix(raw, "feed/") {
			raw = strings.TrimPrefix(raw, "feed/")
		}
		if unescaped, err := url.QueryUnescape(raw); err == nil && strings.TrimSpace(unescaped) != "" {
			raw = unescaped
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		var title *string
		if v := strings.TrimSpace(s.Title); v != "" {
			title = &v
		}
		out = append(out, opmlURLTitle{URL: raw, Title: title})
	}
	return out, nil
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
	anthropicSourceSuggestionModel := h.getUserAnthropicSourceSuggestionModel(r.Context(), userID)
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
		h.expandSourceSuggestionsWithLLMSeeds(r.Context(), userID, sources, preferredTopics, registered, cands, anthropicAPIKey, anthropicSourceSuggestionModel)
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
	llmMeta := h.rankSourceSuggestionsWithLLM(r.Context(), userID, sources, preferredTopics, out, anthropicAPIKey, anthropicSourceSuggestionModel)
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
	model *string,
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
	resp, err := h.worker.RankFeedSuggestionsWithModel(ctx, existing, preferredTopics, cands, anthropicAPIKey, model)
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

func (h *SourceHandler) getUserAnthropicSourceSuggestionModel(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil {
		return nil
	}
	settings, err := h.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil
	}
	if settings.AnthropicSourceSuggestModel == nil || strings.TrimSpace(*settings.AnthropicSourceSuggestModel) == "" {
		return nil
	}
	v := strings.TrimSpace(*settings.AnthropicSourceSuggestModel)
	return &v
}

func (h *SourceHandler) expandSourceSuggestionsWithLLMSeeds(
	ctx context.Context,
	userID string,
	sources []model.Source,
	preferredTopics []string,
	registered map[string]bool,
	cands map[string]*sourceSuggestionAgg,
	anthropicAPIKey *string,
	model *string,
) {
	existing := make([]service.RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, s := range sources {
		existing = append(existing, service.RankFeedSuggestionsExistingSource{URL: s.URL, Title: s.Title})
	}
	resp, err := h.worker.SuggestFeedSeedSitesWithModel(ctx, existing, preferredTopics, anthropicAPIKey, model)
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
