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

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
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

func (h *SourceHandler) Recommended(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 8)
	if limit < 1 || limit > 30 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.RecommendedByUser(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"items": rows,
		"limit": limit,
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
	limit := parseIntOrDefault(q.Get("limit"), 24)
	if limit < 1 || limit > 60 {
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
	googleAPIKey := h.getUserGoogleAPIKey(r.Context(), userID)
	groqAPIKey := h.getUserGroqAPIKey(r.Context(), userID)
	deepseekAPIKey := h.getUserDeepSeekAPIKey(r.Context(), userID)
	openAIAPIKey := h.getUserOpenAIAPIKey(r.Context(), userID)
	anthropicSourceSuggestionModel := h.getUserSourceSuggestionModel(r.Context(), userID)
	anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, openAIAPIKey, anthropicSourceSuggestionModel = selectSourceSuggestionLLM(
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		openAIAPIKey,
		anthropicSourceSuggestionModel,
	)
	var preferredTopics []string
	if h.itemRepo != nil {
		if topics, err := h.itemRepo.PositiveFeedbackTopics(r.Context(), userID, 8); err == nil {
			preferredTopics = topics
		}
	}
	positiveExamples, negativeExamples := h.buildSourceSuggestionFewShotExamples(r.Context(), userID)

	registered := map[string]bool{}
	startAt := time.Now()
	const suggestionMaxLatency = 12 * time.Second
	isOverBudget := func() bool { return time.Since(startAt) >= suggestionMaxLatency }
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
	if len(probes) > 12 {
		probes = probes[:12]
	}

	cands := map[string]*sourceSuggestionAgg{}
	aiReady := (anthropicAPIKey != nil || googleAPIKey != nil || groqAPIKey != nil || deepseekAPIKey != nil || openAIAPIKey != nil) && h.worker != nil
	// AI主導: まずAIシード提案から候補を作る。
	if aiReady {
		h.expandSourceSuggestionsWithLLMSeeds(
			r.Context(),
			userID,
			sources,
			preferredTopics,
			positiveExamples,
			negativeExamples,
			registered,
			cands,
			anthropicAPIKey,
			googleAPIKey,
			groqAPIKey,
			deepseekAPIKey,
			openAIAPIKey,
			anthropicSourceSuggestionModel,
		)
	}
	// ルールベース探索は不足時のみ補完として使う。
	probeFallbackThreshold := limit * 2
	if probeFallbackThreshold < 12 {
		probeFallbackThreshold = 12
	}
	if !aiReady || len(cands) < probeFallbackThreshold {
		for _, p := range probes {
			if isOverBudget() {
				break
			}
			ctx, cancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
			feeds, err := discoverRSSFeeds(ctx, p.ProbeURL)
			cancel()
			if err != nil {
				continue
			}
			for _, f := range feeds {
				if isOverBudget() {
					break
				}
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
	// ルールベースの一次スコアはプール生成までに使い、最終選抜はAIに委ねる。
	// ただしトークンコストを抑えるため、AIへ渡す候補は最大 N 件に制限する。
	poolLimit := limit * 6
	if poolLimit < 24 {
		poolLimit = 24
	}
	if poolLimit > 120 {
		poolLimit = 120
	}
	if len(rows) > poolLimit {
		rows = rows[:poolLimit]
	}
	for _, r := range rows {
		out = append(out, r.row)
	}
	llmMeta := h.rankSourceSuggestionsWithLLM(
		r.Context(),
		userID,
		sources,
		preferredTopics,
		positiveExamples,
		negativeExamples,
		out,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		openAIAPIKey,
		anthropicSourceSuggestionModel,
	)
	if len(out) > limit {
		out = out[:limit]
	}
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
	positiveExamples []service.RankFeedSuggestionsExample,
	negativeExamples []service.RankFeedSuggestionsExample,
	suggestions []sourceSuggestionResponse,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	openAIAPIKey *string,
	model *string,
) map[string]any {
	if h.worker == nil || len(suggestions) == 0 {
		return nil
	}
	hasAnthropic := anthropicAPIKey != nil && strings.TrimSpace(*anthropicAPIKey) != ""
	hasGoogle := googleAPIKey != nil && strings.TrimSpace(*googleAPIKey) != ""
	hasGroq := groqAPIKey != nil && strings.TrimSpace(*groqAPIKey) != ""
	hasDeepSeek := deepseekAPIKey != nil && strings.TrimSpace(*deepseekAPIKey) != ""
	hasOpenAI := openAIAPIKey != nil && strings.TrimSpace(*openAIAPIKey) != ""
	if !hasAnthropic && !hasGoogle && !hasGroq && !hasDeepSeek && !hasOpenAI {
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
	byID := map[string]*sourceSuggestionResponse{}
	for i := range suggestions {
		s := suggestions[i]
		id := fmt.Sprintf("c%03d", i+1)
		cands = append(cands, service.RankFeedSuggestionsCandidate{
			ID:            id,
			URL:           s.URL,
			Title:         s.Title,
			Reasons:       s.Reasons,
			MatchedTopics: s.MatchedTopics,
		})
		byID[id] = &suggestions[i]
	}
	resp, err := h.worker.RankFeedSuggestionsWithModel(
		ctx,
		existing,
		preferredTopics,
		cands,
		positiveExamples,
		negativeExamples,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		openAIAPIKey,
		model,
	)
	if err != nil {
		return map[string]any{
			"error": err.Error(),
			"stage": "rank",
		}
	}
	if resp == nil {
		return map[string]any{
			"error": "empty response from rank-feed-suggestions",
			"stage": "rank",
		}
	}
	h.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	if len(resp.Items) == 0 {
		if resp.LLM != nil {
			return map[string]any{
				"provider":             resp.LLM.Provider,
				"model":                resp.LLM.Model,
				"estimated_cost_usd":   resp.LLM.EstimatedCostUSD,
				"input_tokens":         resp.LLM.InputTokens,
				"output_tokens":        resp.LLM.OutputTokens,
				"pricing_source":       resp.LLM.PricingSource,
				"pricing_model_family": resp.LLM.PricingModelFamily,
				"warning":              "rank returned no items",
				"stage":                "rank",
			}
		}
		return map[string]any{
			"warning": "rank returned no items and no llm meta",
			"stage":   "rank",
		}
	}
	byURL := map[string]*sourceSuggestionResponse{}
	for i := range suggestions {
		byURL[normalizeFeedURL(suggestions[i].URL)] = &suggestions[i]
	}
	rank := make(map[string]int, len(resp.Items))
	for i, it := range resp.Items {
		if it.ID != nil {
			if s, ok := byID[strings.TrimSpace(*it.ID)]; ok {
				reason := strings.TrimSpace(it.Reason)
				if reason != "" {
					s.AIReason = &reason
					s.Reasons = []string{reason}
				}
				conf := it.Confidence
				s.AIConfidence = &conf
				rank[normalizeFeedURL(s.URL)] = i
				continue
			}
		}
		k := normalizeFeedURL(it.URL)
		if k == "" {
			continue
		}
		rank[k] = i
		if s, ok := byURL[k]; ok {
			reason := strings.TrimSpace(it.Reason)
			if reason != "" {
				s.AIReason = &reason
				// 表示理由もAIの説明を主にする。
				s.Reasons = []string{reason}
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
		return map[string]any{
			"warning": "rank succeeded but llm meta is empty",
			"stage":   "rank",
		}
	}
	return map[string]any{
		"provider":             resp.LLM.Provider,
		"model":                resp.LLM.Model,
		"estimated_cost_usd":   resp.LLM.EstimatedCostUSD,
		"input_tokens":         resp.LLM.InputTokens,
		"output_tokens":        resp.LLM.OutputTokens,
		"pricing_source":       resp.LLM.PricingSource,
		"pricing_model_family": resp.LLM.PricingModelFamily,
		"stage":                "rank",
		"items_count":          len(resp.Items),
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

func (h *SourceHandler) getUserSourceSuggestionModel(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil {
		return nil
	}
	settings, err := h.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil
	}
	if settings.SourceSuggestionModel == nil || strings.TrimSpace(*settings.SourceSuggestionModel) == "" {
		return nil
	}
	v := strings.TrimSpace(*settings.SourceSuggestionModel)
	return &v
}

func (h *SourceHandler) getUserGoogleAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetGoogleAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserGroqAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetGroqAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserDeepSeekAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetDeepSeekAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserOpenAIAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetOpenAIAPIKeyEncrypted(ctx, userID)
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

func selectSourceSuggestionLLM(anthropicAPIKey, googleAPIKey, groqAPIKey, deepseekAPIKey, openAIAPIKey, model *string) (*string, *string, *string, *string, *string, *string) {
	hasAnthropic := anthropicAPIKey != nil && strings.TrimSpace(*anthropicAPIKey) != ""
	hasGoogle := googleAPIKey != nil && strings.TrimSpace(*googleAPIKey) != ""
	hasGroq := groqAPIKey != nil && strings.TrimSpace(*groqAPIKey) != ""
	hasDeepSeek := deepseekAPIKey != nil && strings.TrimSpace(*deepseekAPIKey) != ""
	hasOpenAI := openAIAPIKey != nil && strings.TrimSpace(*openAIAPIKey) != ""

	// 明示モデルがある場合は基本的にそのプロバイダを優先。
	// ただし指定プロバイダのキーが無い場合は、利用可能な側へフォールバックして
	// 「AI提案がまったく動かない」状態を避ける。
	if model != nil && strings.TrimSpace(*model) != "" {
		switch service.LLMProviderForModel(model) {
		case "google":
			if hasGoogle {
				return nil, googleAPIKey, nil, nil, nil, model
			}
			if hasAnthropic {
				return anthropicAPIKey, nil, nil, nil, nil, nil
			}
			if hasGroq {
				fallback := "openai/gpt-oss-20b"
				return nil, nil, groqAPIKey, nil, nil, &fallback
			}
			if hasDeepSeek {
				fallback := "deepseek-chat"
				return nil, nil, nil, deepseekAPIKey, nil, &fallback
			}
			if hasOpenAI {
				fallback := "gpt-5-mini"
				return nil, nil, nil, nil, openAIAPIKey, &fallback
			}
			return nil, nil, nil, nil, nil, model
		case "groq":
			if hasGroq {
				return nil, nil, groqAPIKey, nil, nil, model
			}
			if hasAnthropic {
				return anthropicAPIKey, nil, nil, nil, nil, nil
			}
			if hasGoogle {
				fallback := "gemini-2.5-flash"
				return nil, googleAPIKey, nil, nil, nil, &fallback
			}
			if hasDeepSeek {
				fallback := "deepseek-chat"
				return nil, nil, nil, deepseekAPIKey, nil, &fallback
			}
			if hasOpenAI {
				fallback := "gpt-5-mini"
				return nil, nil, nil, nil, openAIAPIKey, &fallback
			}
			return nil, nil, nil, nil, nil, model
		case "deepseek":
			if hasDeepSeek {
				return nil, nil, nil, deepseekAPIKey, nil, model
			}
			if hasAnthropic {
				return anthropicAPIKey, nil, nil, nil, nil, nil
			}
			if hasGoogle {
				fallback := "gemini-2.5-flash"
				return nil, googleAPIKey, nil, nil, nil, &fallback
			}
			if hasGroq {
				fallback := "openai/gpt-oss-20b"
				return nil, nil, groqAPIKey, nil, nil, &fallback
			}
			if hasOpenAI {
				fallback := "gpt-5-mini"
				return nil, nil, nil, nil, openAIAPIKey, &fallback
			}
			return nil, nil, nil, nil, nil, model
		case "openai":
			if hasOpenAI {
				return nil, nil, nil, nil, openAIAPIKey, model
			}
			if hasAnthropic {
				return anthropicAPIKey, nil, nil, nil, nil, nil
			}
			if hasGoogle {
				fallback := "gemini-2.5-flash"
				return nil, googleAPIKey, nil, nil, nil, &fallback
			}
			if hasGroq {
				fallback := "openai/gpt-oss-20b"
				return nil, nil, groqAPIKey, nil, nil, &fallback
			}
			if hasDeepSeek {
				fallback := "deepseek-chat"
				return nil, nil, nil, deepseekAPIKey, nil, &fallback
			}
			return nil, nil, nil, nil, nil, model
		default:
			if hasAnthropic {
				return anthropicAPIKey, nil, nil, nil, nil, model
			}
			if hasGoogle {
				fallback := "gemini-2.5-flash"
				return nil, googleAPIKey, nil, nil, nil, &fallback
			}
			if hasGroq {
				fallback := "openai/gpt-oss-20b"
				return nil, nil, groqAPIKey, nil, nil, &fallback
			}
			if hasDeepSeek {
				fallback := "deepseek-chat"
				return nil, nil, nil, deepseekAPIKey, nil, &fallback
			}
			if hasOpenAI {
				fallback := "gpt-5-mini"
				return nil, nil, nil, nil, openAIAPIKey, &fallback
			}
			return nil, nil, nil, nil, nil, model
		}
	}

	// モデル未指定時は、利用可能なキーに合わせて自動選択。
	if hasAnthropic {
		return anthropicAPIKey, nil, nil, nil, nil, nil
	}
	if hasGoogle {
		fallback := "gemini-2.5-flash"
		return nil, googleAPIKey, nil, nil, nil, &fallback
	}
	if hasGroq {
		fallback := "openai/gpt-oss-20b"
		return nil, nil, groqAPIKey, nil, nil, &fallback
	}
	if hasDeepSeek {
		fallback := "deepseek-chat"
		return nil, nil, nil, deepseekAPIKey, nil, &fallback
	}
	if hasOpenAI {
		fallback := "gpt-5-mini"
		return nil, nil, nil, nil, openAIAPIKey, &fallback
	}
	return nil, nil, nil, nil, nil, nil
}

func (h *SourceHandler) buildSourceSuggestionFewShotExamples(
	ctx context.Context,
	userID string,
) ([]service.RankFeedSuggestionsExample, []service.RankFeedSuggestionsExample) {
	positiveRows, err := h.repo.RecommendedByUser(ctx, userID, 5)
	if err != nil {
		positiveRows = nil
	}
	negativeRows, err := h.repo.LowAffinityByUser(ctx, userID, 3)
	if err != nil {
		negativeRows = nil
	}
	positive := make([]service.RankFeedSuggestionsExample, 0, len(positiveRows))
	for _, row := range positiveRows {
		reason := fmt.Sprintf("読了%d / Fav%d / 直近親和%.2f", row.ReadCount30d, row.FavoriteCount30d, row.AffinityScore)
		positive = append(positive, service.RankFeedSuggestionsExample{
			URL:    row.URL,
			Title:  row.Title,
			Reason: reason,
		})
	}
	negative := make([]service.RankFeedSuggestionsExample, 0, len(negativeRows))
	for _, row := range negativeRows {
		reason := fmt.Sprintf("読了%d / Fav%d / 直近親和%.2f", row.ReadCount30d, row.FavoriteCount30d, row.AffinityScore)
		negative = append(negative, service.RankFeedSuggestionsExample{
			URL:    row.URL,
			Title:  row.Title,
			Reason: reason,
		})
	}
	return positive, negative
}

func (h *SourceHandler) expandSourceSuggestionsWithLLMSeeds(
	ctx context.Context,
	userID string,
	sources []model.Source,
	preferredTopics []string,
	positiveExamples []service.RankFeedSuggestionsExample,
	negativeExamples []service.RankFeedSuggestionsExample,
	registered map[string]bool,
	cands map[string]*sourceSuggestionAgg,
	anthropicAPIKey *string,
	googleAPIKey *string,
	groqAPIKey *string,
	deepseekAPIKey *string,
	openAIAPIKey *string,
	model *string,
) {
	existing := make([]service.RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, s := range sources {
		existing = append(existing, service.RankFeedSuggestionsExistingSource{URL: s.URL, Title: s.Title})
	}
	resp, err := h.worker.SuggestFeedSeedSitesWithModel(
		ctx,
		existing,
		preferredTopics,
		positiveExamples,
		negativeExamples,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		openAIAPIKey,
		model,
	)
	if err != nil || resp == nil {
		return
	}
	h.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	seedItems := resp.Items
	if len(seedItems) > 10 {
		seedItems = seedItems[:10]
	}
	for _, seed := range seedItems {
		seedURL := coerceHTTPURL(strings.TrimSpace(seed.URL))
		if seedURL == "" {
			continue
		}
		addedFromSeed := false
		probeURLs := aiSeedFeedProbeURLs(seedURL)
		if len(probeURLs) == 0 {
			probeURLs = []string{seedURL}
		}
		if len(probeURLs) > 4 {
			probeURLs = probeURLs[:4]
		}
		for _, probe := range probeURLs {
			ctxOne, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			feeds, err := discoverRSSFeeds(ctxOne, probe)
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
					a.Score += 6
				}
				for _, topic := range preferredTopics {
					if sourceSuggestionTopicMatch(f, topic) && !a.MatchedTopics[topic] {
						a.MatchedTopics[topic] = true
						a.Score += 3
					}
				}
				addedFromSeed = true
			}
		}
		// Feed検出に失敗しても、AIシードURL自体を候補として残す。
		// 登録時に再discoverを試みる前提で、候補ゼロ化を防ぐ。
		if !addedFromSeed {
			key := normalizeFeedURL(seedURL)
			if key == "" || registered[key] {
				continue
			}
			a := cands[key]
			if a == nil {
				a = &sourceSuggestionAgg{
					URL:           seedURL,
					Title:         nil,
					Reasons:       map[string]bool{},
					MatchedTopics: map[string]bool{},
					SeedSourceIDs: map[string]bool{},
				}
				cands[key] = a
			}
			reason := "AI提案サイト（登録時にFeed検出）"
			if strings.TrimSpace(seed.Reason) != "" {
				reason = "AI候補: " + strings.TrimSpace(seed.Reason)
			}
			if !a.Reasons[reason] {
				a.Reasons[reason] = true
				a.Score += 4
			}
		}
	}
}

func aiSeedFeedProbeURLs(raw string) []string {
	u, err := url.Parse(coerceHTTPURL(strings.TrimSpace(raw)))
	if err != nil || u.Host == "" {
		return nil
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	base := &url.URL{Scheme: u.Scheme, Host: u.Host}
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	// First try the seed itself as-is.
	add(u.String())
	// Then try common feed endpoints on root.
	for _, p := range []string{"/feed", "/rss", "/atom.xml", "/feed.xml", "/rss.xml", "/index.xml"} {
		c := *base
		c.Path = p
		add(c.String())
	}
	return out
}

func coerceHTTPURL(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err == nil && u.Host != "" {
		if u.Scheme == "" {
			u.Scheme = "https"
		}
		return u.String()
	}
	// Handle hostname-like text without scheme (e.g. "example.com")
	if !strings.Contains(v, "://") && strings.Contains(v, ".") && !strings.ContainsAny(v, " \t\r\n") {
		return "https://" + v
	}
	return ""
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
