package handler

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
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
	repo                   *repository.SourceRepo
	itemRepo               *repository.ItemRepo
	sourceOptimizationRepo *repository.SourceOptimizationRepo
	settingsRepo           *repository.UserSettingsRepo
	llmUsageRepo           *repository.LLMUsageLogRepo
	worker                 *service.WorkerClient
	cipher                 *service.SecretCipher
	publisher              *service.EventPublisher
	cache                  service.JSONCache
}

func NewSourceHandler(
	repo *repository.SourceRepo,
	itemRepo *repository.ItemRepo,
	sourceOptimizationRepo *repository.SourceOptimizationRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	worker *service.WorkerClient,
	cipher *service.SecretCipher,
	publisher *service.EventPublisher,
	cache service.JSONCache,
) *SourceHandler {
	return &SourceHandler{
		repo:                   repo,
		itemRepo:               itemRepo,
		sourceOptimizationRepo: sourceOptimizationRepo,
		settingsRepo:           settingsRepo,
		llmUsageRepo:           llmUsageRepo,
		worker:                 worker,
		cipher:                 cipher,
		publisher:              publisher,
		cache:                  cache,
	}
}

func (h *SourceHandler) Optimization(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if h.sourceOptimizationRepo == nil {
		http.Error(w, "source optimization unavailable", http.StatusInternalServerError)
		return
	}
	sources, err := h.repo.List(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	windowEnd := time.Now()
	windowStart := windowEnd.AddDate(0, 0, -30)
	type optimizationItem struct {
		SourceID       string                               `json:"source_id"`
		Recommendation string                               `json:"recommendation"`
		Reason         string                               `json:"reason"`
		Metrics        repository.SourceOptimizationMetrics `json:"metrics"`
	}
	out := make([]optimizationItem, 0, len(sources))
	for _, source := range sources {
		metrics, err := h.sourceOptimizationRepo.CollectMetrics(r.Context(), userID, source.ID, windowStart)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		decision := service.ClassifySourceOptimization(service.SourceOptimizationMetrics{
			UnreadBacklog:        metrics.UnreadBacklog,
			ReadRate:             metrics.ReadRate,
			FavoriteRate:         metrics.FavoriteRate,
			NotificationOpenRate: metrics.NotificationOpenRate,
			AverageSummaryScore:  metrics.AverageSummaryScore,
		})
		_ = h.sourceOptimizationRepo.InsertSnapshot(r.Context(), userID, source.ID, windowStart, windowEnd, metrics, decision.Recommendation, decision.Reason)
		out = append(out, optimizationItem{SourceID: source.ID, Recommendation: decision.Recommendation, Reason: decision.Reason, Metrics: metrics})
	}
	writeJSON(w, map[string]any{"items": out})
}

func (h *SourceHandler) Navigator(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"

	if h.settingsRepo == nil {
		writeJSON(w, model.SourceNavigatorEnvelope{})
		return
	}
	settings, err := h.settingsRepo.EnsureDefaults(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if settings == nil || !settings.NavigatorEnabled {
		writeJSON(w, model.SourceNavigatorEnvelope{})
		return
	}
	persona := selectBriefingNavigatorPersona(r.Context(), h.cache, userID, settings)
	modelName := resolveBriefingNavigatorModel(settings)
	resolvedModel := ""
	if modelName != nil {
		resolvedModel = strings.TrimSpace(*modelName)
	}
	cacheKey := cacheKeySourceNavigator(userID, persona, resolvedModel)
	if h.cache != nil && !cacheBust {
		var cached model.SourceNavigatorEnvelope
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			if cached.Navigator != nil && strings.TrimSpace(cached.Navigator.Overview) != "" {
				writeJSON(w, cached)
				return
			}
		}
	}

	navigator := h.buildSourceNavigator(r.Context(), userID, time.Now(), persona)
	resp := model.SourceNavigatorEnvelope{Navigator: navigator}
	if h.cache != nil && navigator != nil && strings.TrimSpace(navigator.Overview) != "" {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, briefingNavigatorCacheTTL); err != nil {
			log.Printf("source navigator cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	if navigator != nil && strings.TrimSpace(navigator.Overview) != "" {
		rememberBriefingNavigatorPersona(r.Context(), h.cache, userID, persona)
	}
	writeJSON(w, resp)
}

func (h *SourceHandler) buildSourceNavigator(ctx context.Context, userID string, generatedAt time.Time, persona string) *model.SourceNavigator {
	if h.repo == nil || h.settingsRepo == nil || h.worker == nil || h.cipher == nil {
		return nil
	}
	settings, err := h.settingsRepo.EnsureDefaults(ctx, userID)
	if err != nil {
		log.Printf("source navigator settings user=%s: %v", userID, err)
		return nil
	}
	if settings == nil || !settings.NavigatorEnabled {
		return nil
	}
	modelName := resolveBriefingNavigatorModel(settings)
	if modelName == nil {
		return nil
	}
	candidates, err := h.repo.NavigatorCandidates30d(ctx, userID)
	if err != nil {
		log.Printf("source navigator candidates user=%s: %v", userID, err)
		return nil
	}

	anthropicKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetAnthropicAPIKeyEncrypted, h.cipher, userID, "")
	googleKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetGoogleAPIKeyEncrypted, h.cipher, userID, "")
	groqKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetGroqAPIKeyEncrypted, h.cipher, userID, "")
	fireworksKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetFireworksAPIKeyEncrypted, h.cipher, userID, "")
	deepseekKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetDeepSeekAPIKeyEncrypted, h.cipher, userID, "")
	alibabaKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetAlibabaAPIKeyEncrypted, h.cipher, userID, "")
	mistralKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetMistralAPIKeyEncrypted, h.cipher, userID, "")
	moonshotKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetMoonshotAPIKeyEncrypted, h.cipher, userID, "")
	xaiKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetXAIAPIKeyEncrypted, h.cipher, userID, "")
	zaiKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetZAIAPIKeyEncrypted, h.cipher, userID, "")
	openRouterKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetOpenRouterAPIKeyEncrypted, h.cipher, userID, "")
	poeKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetPoeAPIKeyEncrypted, h.cipher, userID, "")
	siliconFlowKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetSiliconFlowAPIKeyEncrypted, h.cipher, userID, "")
	openAIKey, _ := loadAndDecryptUserSecret(ctx, h.settingsRepo.GetOpenAIAPIKeyEncrypted, h.cipher, userID, "")
	switch service.LLMProviderForModel(modelName) {
	case "openrouter":
		openAIKey = openRouterKey
	case "moonshot":
		openAIKey = moonshotKey
	case "poe":
		openAIKey = poeKey
	case "siliconflow":
		openAIKey = siliconFlowKey
	}

	workerCandidates := make([]service.SourceNavigatorCandidate, 0, len(candidates))
	titleBySourceID := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		var lastFetchedAt *string
		if candidate.LastFetchedAt != nil {
			v := candidate.LastFetchedAt.Format(time.RFC3339)
			lastFetchedAt = &v
		}
		var lastItemAt *string
		if candidate.LastItemAt != nil {
			v := candidate.LastItemAt.Format(time.RFC3339)
			lastItemAt = &v
		}
		titleBySourceID[candidate.SourceID] = candidate.Title
		workerCandidates = append(workerCandidates, service.SourceNavigatorCandidate{
			SourceID:               candidate.SourceID,
			Title:                  candidate.Title,
			URL:                    candidate.URL,
			Enabled:                candidate.Enabled,
			Status:                 candidate.Status,
			LastFetchedAt:          lastFetchedAt,
			LastItemAt:             lastItemAt,
			TotalItems30d:          candidate.TotalItems30d,
			UnreadItems30d:         candidate.UnreadItems30d,
			ReadItems30d:           candidate.ReadItems30d,
			FavoriteCount30d:       candidate.FavoriteCount30d,
			AvgItemsPerDay30d:      candidate.AvgItemsPerDay30d,
			ActiveDays30d:          candidate.ActiveDays30d,
			AvgItemsPerActiveDay30: candidate.AvgItemsPerActiveDay30,
			FailureRate:            candidate.FailureRate,
		})
	}

	workerCtx := service.WithWorkerTraceMetadata(ctx, "source_navigator", &userID, nil, nil, nil)
	resp, err := h.worker.GenerateSourceNavigatorWithModel(
		workerCtx,
		persona,
		workerCandidates,
		derefString(anthropicKey),
		derefString(googleKey),
		derefString(groqKey),
		derefString(deepseekKey),
		derefString(alibabaKey),
		derefString(mistralKey),
		derefString(xaiKey),
		derefString(zaiKey),
		derefString(fireworksKey),
		derefString(openAIKey),
		modelName,
	)
	if err != nil {
		log.Printf("source navigator worker user=%s model=%s: %v", userID, strings.TrimSpace(*modelName), err)
		return nil
	}
	recordAskLLMUsage(ctx, h.llmUsageRepo, h.cache, "source_navigator", resp.LLM, &userID)
	if strings.TrimSpace(resp.Overview) == "" {
		return nil
	}
	meta := briefingNavigatorPersonaMeta(persona)
	return &model.SourceNavigator{
		Enabled:        true,
		Persona:        persona,
		CharacterName:  meta.CharacterName,
		CharacterTitle: meta.CharacterTitle,
		AvatarStyle:    meta.AvatarStyle,
		SpeechStyle:    meta.SpeechStyle,
		Overview:       strings.TrimSpace(resp.Overview),
		Keep:           mapSourceNavigatorPicks(resp.Keep, titleBySourceID),
		Watch:          mapSourceNavigatorPicks(resp.Watch, titleBySourceID),
		Standout:       mapSourceNavigatorPicks(resp.Standout, titleBySourceID),
		GeneratedAt:    &generatedAt,
	}
}

func mapSourceNavigatorPicks(in []service.SourceNavigatorPick, titleBySourceID map[string]string) []model.SourceNavigatorPick {
	out := make([]model.SourceNavigatorPick, 0, len(in))
	seen := map[string]bool{}
	for _, row := range in {
		sourceID := strings.TrimSpace(row.SourceID)
		if sourceID == "" || seen[sourceID] {
			continue
		}
		title := strings.TrimSpace(row.Title)
		if title == "" {
			title = strings.TrimSpace(titleBySourceID[sourceID])
		}
		comment := strings.TrimSpace(row.Comment)
		if title == "" || comment == "" {
			continue
		}
		out = append(out, model.SourceNavigatorPick{
			SourceID: sourceID,
			Title:    title,
			Comment:  comment,
		})
		seen[sourceID] = true
	}
	return out
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

func (h *SourceHandler) ItemStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.repo.ItemStatsByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"items": rows,
	})
}

func (h *SourceHandler) DailyStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 30)
	rows, err := h.repo.DailyStatsByUser(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"items":    rows,
		"overview": repository.BuildSourcesDailyOverview(rows),
	})
}

func (h *SourceHandler) Recommended(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 8)
	if limit < 1 || limit > 30 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	out, llmMeta, err := h.buildSourceRecommendations(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": out, "limit": limit, "llm": llmMeta})
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
	if err := h.publisher.SendSearchSuggestionSourceUpsertE(r.Context(), s.ID); err != nil {
		log.Printf("search suggestion source upsert enqueue failed source_id=%s err=%v", s.ID, err)
	}

	// For one-off URLs, seed an item immediately and trigger async processing.
	if strings.EqualFold(body.Type, "manual") && h.itemRepo != nil {
		itemID, created, err := h.itemRepo.UpsertFromFeed(r.Context(), s.ID, body.URL, body.Title)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if created {
			h.publisher.SendItemCreatedWithReasonE(r.Context(), itemID, s.ID, body.URL, body.Title, "manual_source")
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

type probeSeed struct {
	SourceID string
	ProbeURL string
	Reason   string
}

func llmUsageMetaMap(llm *service.LLMUsage, stage string) map[string]any {
	llm = service.NormalizeCatalogPricedUsage("source_suggestion", llm)
	if llm == nil {
		if stage == "" {
			return nil
		}
		return map[string]any{"stage": stage}
	}
	meta := map[string]any{
		"provider":             llm.Provider,
		"model":                llm.Model,
		"requested_model":      llm.RequestedModel,
		"resolved_model":       llm.ResolvedModel,
		"estimated_cost_usd":   llm.EstimatedCostUSD,
		"input_tokens":         llm.InputTokens,
		"output_tokens":        llm.OutputTokens,
		"pricing_source":       llm.PricingSource,
		"pricing_model_family": llm.PricingModelFamily,
	}
	if stage != "" {
		meta["stage"] = stage
	}
	return meta
}

func (h *SourceHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	limit := parseIntOrDefault(q.Get("limit"), 24)
	if limit < 1 || limit > 60 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	out, llmMeta, err := h.buildSourceRecommendations(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": out, "limit": limit, "llm": llmMeta})
}

func (h *SourceHandler) buildSourceRecommendations(ctx context.Context, userID string, limit int) ([]sourceSuggestionResponse, map[string]any, error) {
	sources, err := h.repo.List(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if len(sources) == 0 {
		return []sourceSuggestionResponse{}, nil, nil
	}
	anthropicAPIKey := h.getUserAnthropicAPIKey(ctx, userID)
	googleAPIKey := h.getUserGoogleAPIKey(ctx, userID)
	groqAPIKey := h.getUserGroqAPIKey(ctx, userID)
	fireworksAPIKey := h.getUserFireworksAPIKey(ctx, userID)
	deepseekAPIKey := h.getUserDeepSeekAPIKey(ctx, userID)
	alibabaAPIKey := h.getUserAlibabaAPIKey(ctx, userID)
	mistralAPIKey := h.getUserMistralAPIKey(ctx, userID)
	xaiAPIKey := h.getUserXAIAPIKey(ctx, userID)
	zaiAPIKey := h.getUserZAIAPIKey(ctx, userID)
	openRouterAPIKey := h.getUserOpenRouterAPIKey(ctx, userID)
	poeAPIKey := h.getUserPoeAPIKey(ctx, userID)
	siliconFlowAPIKey := h.getUserSiliconFlowAPIKey(ctx, userID)
	openAIAPIKey := h.getUserOpenAIAPIKey(ctx, userID)
	anthropicSourceSuggestionModel := h.getUserSourceSuggestionModel(ctx, userID)
	anthropicAPIKey, googleAPIKey, groqAPIKey, fireworksAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, openAIAPIKey, anthropicSourceSuggestionModel = selectSourceSuggestionLLM(
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		fireworksAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		openRouterAPIKey,
		poeAPIKey,
		siliconFlowAPIKey,
		openAIAPIKey,
		anthropicSourceSuggestionModel,
	)
	var preferredTopics []string
	if h.itemRepo != nil {
		if topics, err := h.itemRepo.PositiveFeedbackTopics(ctx, userID, 8); err == nil {
			preferredTopics = topics
		}
	}
	positiveExamples, negativeExamples := h.buildSourceSuggestionFewShotExamples(ctx, userID)

	registered := map[string]bool{}
	startAt := time.Now()
	const suggestionMaxLatency = 60 * time.Second
	isOverBudget := func() bool { return time.Since(startAt) >= suggestionMaxLatency }
	remainingSuggestionBudget := func() time.Duration {
		if d := suggestionMaxLatency - time.Since(startAt); d > 0 {
			return d
		}
		return 0
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
	if len(probes) > 12 {
		probes = probes[:12]
	}

	cands := map[string]*sourceSuggestionAgg{}
	aiReady := (anthropicAPIKey != nil || googleAPIKey != nil || groqAPIKey != nil || fireworksAPIKey != nil || deepseekAPIKey != nil || alibabaAPIKey != nil || mistralAPIKey != nil || xaiAPIKey != nil || zaiAPIKey != nil || openAIAPIKey != nil) && h.worker != nil
	var seedLLMMeta map[string]any
	timedOutInAiStep := false
	if aiReady {
		seedLLMMeta, timedOutInAiStep = h.expandSourceSuggestionsWithLLMSeeds(
			ctx,
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
			fireworksAPIKey,
			deepseekAPIKey,
			alibabaAPIKey,
			mistralAPIKey,
			xaiAPIKey,
			zaiAPIKey,
			openAIAPIKey,
			anthropicSourceSuggestionModel,
			remainingSuggestionBudget,
		)
	}
	if !aiReady {
		populateSourceSuggestionsFromProbes(ctx, probes, preferredTopics, registered, cands, remainingSuggestionBudget, discoverRSSFeeds)
	}

	out := make([]sourceSuggestionResponse, 0, len(cands))
	type sortable struct {
		row   sourceSuggestionResponse
		score int
	}
	rows := make([]sortable, 0, len(cands))
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
		ctx,
		userID,
		sources,
		preferredTopics,
		positiveExamples,
		negativeExamples,
		out,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		fireworksAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		openAIAPIKey,
		anthropicSourceSuggestionModel,
		remainingSuggestionBudget,
	)
	if llmMeta == nil && seedLLMMeta != nil {
		llmMeta = seedLLMMeta
	}
	if isOverBudget() {
		llmMeta = mergeLLMWarning(llmMeta, "source suggestion timed out; partial results returned", "timeout")
	}
	if timedOutInAiStep {
		llmMeta = mergeLLMWarning(llmMeta, "source suggestion timed out during AI seed generation", "seed_generation")
	}
	if anthropicSourceSuggestionModel != nil && strings.TrimSpace(*anthropicSourceSuggestionModel) != "" {
		if llmMeta == nil {
			llmMeta = map[string]any{}
		}
		if _, ok := llmMeta["requested_model"]; !ok {
			llmMeta["requested_model"] = strings.TrimSpace(*anthropicSourceSuggestionModel)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, llmMeta, nil
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

func populateSourceSuggestionsFromProbes(
	ctx context.Context,
	probes []probeSeed,
	preferredTopics []string,
	registered map[string]bool,
	cands map[string]*sourceSuggestionAgg,
	remainingSuggestionBudget func() time.Duration,
	discover func(context.Context, string) ([]FeedCandidate, error),
) {
	if remainingSuggestionBudget == nil {
		remainingSuggestionBudget = func() time.Duration { return 0 }
	}
	if discover == nil {
		return
	}
	for _, p := range probes {
		probeTimeout := capDuration(1200*time.Millisecond, remainingSuggestionBudget())
		if probeTimeout <= 0 {
			break
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		feeds, err := discover(probeCtx, p.ProbeURL)
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
				if sourceSuggestionTopicMatch(f, topic) && !a.MatchedTopics[topic] {
					a.MatchedTopics[topic] = true
					a.Score += 3
				}
			}
		}
	}
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
	fireworksAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	openAIAPIKey *string,
	model *string,
	remainingSuggestionBudget func() time.Duration,
) map[string]any {
	if h.worker == nil || len(suggestions) == 0 {
		return nil
	}
	if remainingSuggestionBudget == nil {
		remainingSuggestionBudget = func() time.Duration { return 0 }
	}
	if remainingSuggestionBudget() <= 0 {
		return map[string]any{
			"warning": "source suggestion rank skipped due timeout budget",
			"stage":   "rank",
		}
	}
	hasAnthropic := anthropicAPIKey != nil && strings.TrimSpace(*anthropicAPIKey) != ""
	hasGoogle := googleAPIKey != nil && strings.TrimSpace(*googleAPIKey) != ""
	hasGroq := groqAPIKey != nil && strings.TrimSpace(*groqAPIKey) != ""
	hasFireworks := fireworksAPIKey != nil && strings.TrimSpace(*fireworksAPIKey) != ""
	hasDeepSeek := deepseekAPIKey != nil && strings.TrimSpace(*deepseekAPIKey) != ""
	hasAlibaba := alibabaAPIKey != nil && strings.TrimSpace(*alibabaAPIKey) != ""
	hasMistral := mistralAPIKey != nil && strings.TrimSpace(*mistralAPIKey) != ""
	hasXAI := xaiAPIKey != nil && strings.TrimSpace(*xaiAPIKey) != ""
	hasZAI := zaiAPIKey != nil && strings.TrimSpace(*zaiAPIKey) != ""
	hasOpenAI := openAIAPIKey != nil && strings.TrimSpace(*openAIAPIKey) != ""
	if !hasAnthropic && !hasGoogle && !hasGroq && !hasFireworks && !hasDeepSeek && !hasAlibaba && !hasMistral && !hasXAI && !hasZAI && !hasOpenAI {
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
	rankBudget := capDuration(20*time.Second, remainingSuggestionBudget())
	if rankBudget <= 0 {
		return map[string]any{
			"warning": "source suggestion rank skipped due timeout budget",
			"stage":   "rank",
		}
	}
	rankCtx, cancel := context.WithTimeout(ctx, rankBudget)
	resp, err := h.worker.RankFeedSuggestionsWithModel(
		rankCtx,
		existing,
		preferredTopics,
		cands,
		positiveExamples,
		negativeExamples,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		fireworksAPIKey,
		openAIAPIKey,
		model,
	)
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return map[string]any{
				"warning": "source suggestion ranking timed out",
				"stage":   "rank",
			}
		}
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
	resp.LLM = service.NormalizeCatalogPricedUsage("source_suggestion", resp.LLM)
	h.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	if len(resp.Items) == 0 {
		if resp.LLM != nil {
			return map[string]any{
				"provider":             resp.LLM.Provider,
				"model":                resp.LLM.Model,
				"requested_model":      resp.LLM.RequestedModel,
				"resolved_model":       resp.LLM.ResolvedModel,
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
		"requested_model":      resp.LLM.RequestedModel,
		"resolved_model":       resp.LLM.ResolvedModel,
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

func (h *SourceHandler) getUserAlibabaAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetAlibabaAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserMistralAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetMistralAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserXAIAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetXAIAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserZAIAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetZAIAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserFireworksAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetFireworksAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserOpenRouterAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetOpenRouterAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserPoeAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetPoeAPIKeyEncrypted(ctx, userID)
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

func (h *SourceHandler) getUserSiliconFlowAPIKey(ctx context.Context, userID string) *string {
	if h.settingsRepo == nil || h.cipher == nil {
		return nil
	}
	enc, err := h.settingsRepo.GetSiliconFlowAPIKeyEncrypted(ctx, userID)
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

func selectSourceSuggestionLLM(anthropicAPIKey, googleAPIKey, groqAPIKey, fireworksAPIKey, deepseekAPIKey, alibabaAPIKey, mistralAPIKey, xaiAPIKey, zaiAPIKey, openRouterAPIKey, poeAPIKey, siliconFlowAPIKey, openAIAPIKey, model *string) (*string, *string, *string, *string, *string, *string, *string, *string, *string, *string, *string) {
	hasAnthropic := anthropicAPIKey != nil && strings.TrimSpace(*anthropicAPIKey) != ""
	hasGoogle := googleAPIKey != nil && strings.TrimSpace(*googleAPIKey) != ""
	hasGroq := groqAPIKey != nil && strings.TrimSpace(*groqAPIKey) != ""
	hasFireworks := fireworksAPIKey != nil && strings.TrimSpace(*fireworksAPIKey) != ""
	hasDeepSeek := deepseekAPIKey != nil && strings.TrimSpace(*deepseekAPIKey) != ""
	hasAlibaba := alibabaAPIKey != nil && strings.TrimSpace(*alibabaAPIKey) != ""
	hasMistral := mistralAPIKey != nil && strings.TrimSpace(*mistralAPIKey) != ""
	hasXAI := xaiAPIKey != nil && strings.TrimSpace(*xaiAPIKey) != ""
	hasZAI := zaiAPIKey != nil && strings.TrimSpace(*zaiAPIKey) != ""
	hasOpenRouter := openRouterAPIKey != nil && strings.TrimSpace(*openRouterAPIKey) != ""
	hasPoe := poeAPIKey != nil && strings.TrimSpace(*poeAPIKey) != ""
	hasSiliconFlow := siliconFlowAPIKey != nil && strings.TrimSpace(*siliconFlowAPIKey) != ""
	hasOpenAI := openAIAPIKey != nil && strings.TrimSpace(*openAIAPIKey) != ""
	purpose := "source_suggestion"

	selectByProvider := func(provider string, explicitModel *string) (*string, *string, *string, *string, *string, *string, *string, *string, *string, *string, *string) {
		resolved := explicitModel
		if resolved == nil || strings.TrimSpace(*resolved) == "" {
			v := service.DefaultLLMModelForPurpose(provider, purpose)
			resolved = &v
		}
		switch provider {
		case "google":
			if hasGoogle {
				return nil, googleAPIKey, nil, nil, nil, nil, nil, nil, nil, nil, resolved
			}
		case "groq":
			if hasGroq {
				return nil, nil, groqAPIKey, nil, nil, nil, nil, nil, nil, nil, resolved
			}
		case "fireworks":
			if hasFireworks {
				return nil, nil, nil, fireworksAPIKey, nil, nil, nil, nil, nil, nil, resolved
			}
		case "deepseek":
			if hasDeepSeek {
				return nil, nil, nil, nil, deepseekAPIKey, nil, nil, nil, nil, nil, resolved
			}
		case "alibaba":
			if hasAlibaba {
				return nil, nil, nil, nil, nil, alibabaAPIKey, nil, nil, nil, nil, resolved
			}
		case "mistral":
			if hasMistral {
				return nil, nil, nil, nil, nil, nil, mistralAPIKey, nil, nil, nil, resolved
			}
		case "xai":
			if hasXAI {
				return nil, nil, nil, nil, nil, nil, nil, xaiAPIKey, nil, nil, resolved
			}
		case "zai":
			if hasZAI {
				return nil, nil, nil, nil, nil, nil, nil, nil, zaiAPIKey, nil, resolved
			}
		case "openrouter":
			if hasOpenRouter {
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, openRouterAPIKey, resolved
			}
		case "poe":
			if hasPoe {
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, poeAPIKey, resolved
			}
		case "siliconflow":
			if hasSiliconFlow {
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, siliconFlowAPIKey, resolved
			}
		case "openai":
			if hasOpenAI {
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, openAIAPIKey, resolved
			}
		case "anthropic":
			if hasAnthropic {
				return anthropicAPIKey, nil, nil, nil, nil, nil, nil, nil, nil, nil, resolved
			}
		}
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
	}

	// 明示モデルがある場合は基本的にそのプロバイダを優先。
	// ただし指定プロバイダのキーが無い場合は、利用可能な側へフォールバックして
	// 「AI提案がまったく動かない」状態を避ける。
	if model != nil && strings.TrimSpace(*model) != "" {
		preferredProvider := service.LLMProviderForModel(model)
		if outAnthropic, outGoogle, outGroq, outFireworks, outDeepSeek, outAlibaba, outMistral, outXAI, outZAI, outOpenAI, resolved := selectByProvider(preferredProvider, model); resolved != nil {
			return outAnthropic, outGoogle, outGroq, outFireworks, outDeepSeek, outAlibaba, outMistral, outXAI, outZAI, outOpenAI, resolved
		}
		for _, provider := range service.CostEfficientLLMProviders(preferredProvider) {
			if outAnthropic, outGoogle, outGroq, outFireworks, outDeepSeek, outAlibaba, outMistral, outXAI, outZAI, outOpenAI, resolved := selectByProvider(provider, nil); resolved != nil {
				return outAnthropic, outGoogle, outGroq, outFireworks, outDeepSeek, outAlibaba, outMistral, outXAI, outZAI, outOpenAI, resolved
			}
		}
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, model
	}

	// モデル未指定時は、利用可能なキーに合わせて自動選択。
	for _, provider := range service.CostEfficientLLMProviders("") {
		if outAnthropic, outGoogle, outGroq, outFireworks, outDeepSeek, outAlibaba, outMistral, outXAI, outZAI, outOpenAI, resolved := selectByProvider(provider, nil); resolved != nil {
			return outAnthropic, outGoogle, outGroq, outFireworks, outDeepSeek, outAlibaba, outMistral, outXAI, outZAI, outOpenAI, resolved
		}
	}
	return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
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
	fireworksAPIKey *string,
	deepseekAPIKey *string,
	alibabaAPIKey *string,
	mistralAPIKey *string,
	xaiAPIKey *string,
	zaiAPIKey *string,
	openAIAPIKey *string,
	model *string,
	remainingSuggestionBudget func() time.Duration,
) (map[string]any, bool) {
	if remainingSuggestionBudget == nil {
		remainingSuggestionBudget = func() time.Duration { return 0 }
	}
	if remainingSuggestionBudget() <= 0 {
		return map[string]any{
			"warning": "source suggestion seed skipped due timeout budget",
			"stage":   "seed_generation",
		}, true
	}
	existing := make([]service.RankFeedSuggestionsExistingSource, 0, len(sources))
	for _, s := range sources {
		existing = append(existing, service.RankFeedSuggestionsExistingSource{URL: s.URL, Title: s.Title})
	}
	seedBudget := capDuration(25*time.Second, remainingSuggestionBudget())
	if seedBudget <= 0 {
		return map[string]any{
			"warning": "source suggestion seed skipped due timeout budget",
			"stage":   "seed_generation",
		}, true
	}
	ctxSeed, cancelSeed := context.WithTimeout(ctx, seedBudget)
	defer cancelSeed()
	resp, err := h.worker.SuggestFeedSeedSitesWithModel(
		ctxSeed,
		existing,
		preferredTopics,
		positiveExamples,
		negativeExamples,
		anthropicAPIKey,
		googleAPIKey,
		groqAPIKey,
		deepseekAPIKey,
		alibabaAPIKey,
		mistralAPIKey,
		xaiAPIKey,
		zaiAPIKey,
		fireworksAPIKey,
		openAIAPIKey,
		model,
	)
	if err != nil || resp == nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return map[string]any{
				"warning": "source suggestion seed generation timed out",
				"stage":   "seed_generation",
			}, true
		}
		if err != nil {
			return map[string]any{
				"error": err.Error(),
				"stage": "seed_generation",
			}, false
		}
		return map[string]any{
			"error": "empty response from suggest-feed-seed-sites",
			"stage": "seed_generation",
		}, false
	}
	if remainingSuggestionBudget() <= 0 {
		return map[string]any{
			"warning": "source suggestion seed generation timed out",
			"stage":   "seed_generation",
		}, true
	}
	resp.LLM = service.NormalizeCatalogPricedUsage("source_suggestion", resp.LLM)
	h.recordSourceSuggestionLLMUsage(ctx, userID, resp.LLM)
	meta := llmUsageMetaMap(resp.LLM, "seed_generation")
	if meta == nil {
		meta = map[string]any{"stage": "seed_generation"}
	}
	meta["items_count"] = len(resp.Items)
	seedItems := resp.Items
	if len(seedItems) > 10 {
		seedItems = seedItems[:10]
	}
	beforeCount := len(cands)
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
			if remainingSuggestionBudget() <= 0 {
				break
			}
			probeTimeout := capDuration(1500*time.Millisecond, remainingSuggestionBudget())
			if probeTimeout <= 0 {
				break
			}
			ctxOne, cancel := context.WithTimeout(ctx, probeTimeout)
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
		if remainingSuggestionBudget() <= 0 && !addedFromSeed {
			meta = mergeLLMWarning(meta, "source suggestion timed out during AI seed probing", "seed_generation")
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
					Title:         seed.Title,
					Reasons:       map[string]bool{},
					MatchedTopics: map[string]bool{},
					SeedSourceIDs: map[string]bool{},
				}
				cands[key] = a
			}
			if a.Title == nil && seed.Title != nil && strings.TrimSpace(*seed.Title) != "" {
				a.Title = seed.Title
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
	if len(seedItems) == 0 {
		meta = mergeLLMWarning(meta, "seed returned no items", "seed_generation")
	} else if len(cands) == beforeCount {
		meta = mergeLLMWarning(meta, "seed produced no usable candidates", "seed_generation")
	}
	return meta, false
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

func capDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func mergeLLMWarning(llmMeta map[string]any, warning string, stage string) map[string]any {
	if llmMeta == nil {
		llmMeta = map[string]any{}
	}
	existing, ok := llmMeta["warning"].(string)
	if ok && existing != "" {
		if !strings.Contains(existing, warning) {
			llmMeta["warning"] = fmt.Sprintf("%s; %s", existing, warning)
		}
	} else {
		llmMeta["warning"] = warning
	}
	if stage != "" {
		llmMeta["stage"] = stage
	}
	return llmMeta
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *SourceHandler) recordSourceSuggestionLLMUsage(ctx context.Context, userID string, llm *service.LLMUsage) {
	llm = service.NormalizeCatalogPricedUsage("source_suggestion", llm)
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
		RequestedModel:           llm.RequestedModel,
		ResolvedModel:            llm.ResolvedModel,
		PricingModelFamily:       llm.PricingModelFamily,
		PricingSource:            llm.PricingSource,
		OpenRouterCostUSD:        llm.OpenRouterCostUSD,
		OpenRouterGenerationID:   strings.TrimSpace(llm.OpenRouterGenerationID),
		Purpose:                  "source_suggestion",
		InputTokens:              llm.InputTokens,
		OutputTokens:             llm.OutputTokens,
		CacheCreationInputTokens: llm.CacheCreationInputTokens,
		CacheReadInputTokens:     llm.CacheReadInputTokens,
		EstimatedCostUSD:         llm.EstimatedCostUSD,
	}); err != nil {
		// Best-effort logging: don't fail source suggestions UI on usage log issues.
		return
	}
	_ = service.BumpUserLLMUsageCacheVersion(ctx, h.cache, userID)
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
	if err := h.publisher.SendSearchSuggestionSourceUpsertE(r.Context(), s.ID); err != nil {
		log.Printf("search suggestion source upsert enqueue failed source_id=%s err=%v", s.ID, err)
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
	if err := h.publisher.SendSearchSuggestionSourceDeleteE(r.Context(), id); err != nil {
		log.Printf("search suggestion source delete enqueue failed source_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendSearchSuggestionTopicsRefreshE(r.Context(), userID); err != nil {
		log.Printf("search suggestion topics refresh enqueue failed source_id=%s user_id=%s err=%v", id, userID, err)
	}
	w.WriteHeader(http.StatusNoContent)
}
