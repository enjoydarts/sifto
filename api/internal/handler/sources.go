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
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

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
	keyProvider            *service.UserKeyProvider
	suggestionSvc          *service.SourceSuggestionService
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
	keyProvider *service.UserKeyProvider,
) *SourceHandler {
	h := &SourceHandler{
		repo:                   repo,
		itemRepo:               itemRepo,
		sourceOptimizationRepo: sourceOptimizationRepo,
		settingsRepo:           settingsRepo,
		llmUsageRepo:           llmUsageRepo,
		worker:                 worker,
		cipher:                 cipher,
		publisher:              publisher,
		cache:                  cache,
		keyProvider:            keyProvider,
	}
	h.suggestionSvc = service.NewSourceSuggestionService(
		repo, itemRepo, settingsRepo, llmUsageRepo, worker, cache, keyProvider,
	)
	return h
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
	writeJSON(w, sourceListItemsResponse{Items: out})
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
	if h.repo == nil || h.settingsRepo == nil || h.worker == nil || h.keyProvider == nil {
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

	nk := loadNavigatorKeys(ctx, h.keyProvider, userID, modelName)

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
		derefString(nk.anthropicKey),
		derefString(nk.googleKey),
		derefString(nk.groqKey),
		derefString(nk.deepseekKey),
		derefString(nk.alibabaKey),
		derefString(nk.mistralKey),
		derefString(nk.xaiKey),
		derefString(nk.zaiKey),
		derefString(nk.fireworksKey),
		derefString(nk.openAIKey),
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
	writeJSON(w, sourceListItemsResponse{Items: rows})
}

func (h *SourceHandler) ItemStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.repo.ItemStatsByUser(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, sourceListItemsResponse{Items: rows})
}

func (h *SourceHandler) DailyStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 30)
	rows, err := h.repo.DailyStatsByUser(r.Context(), userID, days)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, sourceDailyStatsResponse{
		Items:    rows,
		Overview: repository.BuildSourcesDailyOverview(rows),
	})
}

func (h *SourceHandler) Recommended(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 8)
	if limit < 1 || limit > 30 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	out, llmMeta, err := h.suggestionSvc.BuildSourceRecommendations(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, sourceRecommendResponse{Items: out, Limit: limit, LLM: llmMeta})
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

func importURLTitlePairs(ctx context.Context, repo *repository.SourceRepo, userID string, pairs []opmlURLTitle) importResultResponse {
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
	return importResultResponse{
		Status:      "ok",
		Total:       len(pairs),
		Added:       added,
		Skipped:     skipped,
		Invalid:     invalid,
		ErrorCount:  len(errorsOut),
		ErrorSample: errorsOut,
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

	feeds, err := service.DiscoverRSSFeeds(r.Context(), strings.TrimSpace(body.URL))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	writeJSON(w, discoverFeedsResponse{Feeds: feeds})
}

func (h *SourceHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	limit := parseIntOrDefault(q.Get("limit"), 24)
	if limit < 1 || limit > 60 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	out, llmMeta, err := h.suggestionSvc.BuildSourceRecommendations(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, sourceRecommendResponse{Items: out, Limit: limit, LLM: llmMeta})
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
