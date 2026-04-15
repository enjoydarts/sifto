package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/go-chi/chi/v5"
)

type ItemHandler struct {
	repo            *repository.ItemRepo
	sourceRepo      *repository.SourceRepo
	readingGoalRepo *repository.ReadingGoalRepo
	streakRepo      *repository.ReadingStreakRepo
	snapshotRepo    *repository.BriefingSnapshotRepo
	prefProfileRepo *repository.PreferenceProfileRepo
	reviewQueueRepo *repository.ReviewQueueRepo
	settingsRepo    *repository.UserSettingsRepo
	llmUsageRepo    *repository.LLMUsageLogRepo
	publisher       *service.EventPublisher
	cipher          *service.SecretCipher
	worker          *service.WorkerClient
	cache           service.JSONCache
	search          *service.MeilisearchService
	searchItems     *service.ItemSearchService
	searchSuggest   *service.SearchSuggestionService
	detail          *service.ItemDetailService
	keyProvider     *service.UserKeyProvider
}

const itemsListCacheTTL = 30 * time.Second
const focusQueueCacheTTL = 60 * time.Second
const triageAllCacheTTL = 90 * time.Second
const relatedItemsCacheTTL = 5 * time.Minute
const itemDetailCacheTTL = 5 * time.Minute

type retryBulkRequest struct {
	ItemIDs []string `json:"item_ids"`
}

type retryBulkCandidate struct {
	ID       string
	SourceID string
	URL      string
}

type retryBulkResult struct {
	Status        string   `json:"status"`
	ItemIDs       []string `json:"item_ids"`
	QueuedCount   int      `json:"queued_count"`
	SkippedCount  int      `json:"skipped_count"`
	queuedItemIDs []string
}

type deleteBulkResult struct {
	Status       string   `json:"status"`
	ItemIDs      []string `json:"item_ids"`
	UpdatedCount int      `json:"updated_count"`
	SkippedCount int      `json:"skipped_count"`
}

func normalizeBulkItemIDs(itemIDs []string) []string {
	if len(itemIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(itemIDs))
	out := make([]string, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		out = append(out, itemID)
	}
	return out
}

func runRetryBulk(
	ctx context.Context,
	itemIDs []string,
	reset func(context.Context, string) (retryBulkCandidate, error),
	enqueue func(context.Context, retryBulkCandidate) error,
) retryBulkResult {
	result := retryBulkResult{
		Status:  "queued",
		ItemIDs: append([]string(nil), itemIDs...),
	}
	for _, itemID := range itemIDs {
		item, err := reset(ctx, itemID)
		if err != nil {
			if !errors.Is(err, repository.ErrConflict) && !errors.Is(err, repository.ErrNotFound) {
				log.Printf("retry bulk reset failed item_id=%s err=%v", itemID, err)
			}
			result.SkippedCount++
			continue
		}
		if err := enqueue(ctx, item); err != nil {
			log.Printf("retry bulk enqueue failed item_id=%s err=%v", item.ID, err)
			result.SkippedCount++
			continue
		}
		result.QueuedCount++
		result.queuedItemIDs = append(result.queuedItemIDs, item.ID)
	}
	return result
}

func runDeleteBulk(
	ctx context.Context,
	itemIDs []string,
	deleteItem func(context.Context, string) error,
) deleteBulkResult {
	result := deleteBulkResult{
		Status:  "ok",
		ItemIDs: append([]string(nil), itemIDs...),
	}
	for _, itemID := range itemIDs {
		if err := deleteItem(ctx, itemID); err != nil {
			if !errors.Is(err, repository.ErrConflict) && !errors.Is(err, repository.ErrNotFound) {
				log.Printf("delete bulk failed item_id=%s err=%v", itemID, err)
			}
			result.SkippedCount++
			continue
		}
		result.UpdatedCount++
	}
	return result
}

func (h *ItemHandler) RetryBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body retryBulkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	itemIDs := normalizeBulkItemIDs(body.ItemIDs)
	if len(itemIDs) == 0 {
		http.Error(w, "item_ids is required", http.StatusBadRequest)
		return
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}

	result := runRetryBulk(
		r.Context(),
		itemIDs,
		func(ctx context.Context, itemID string) (retryBulkCandidate, error) {
			item, err := h.repo.ResetForExtractRetry(ctx, itemID, userID)
			if err != nil {
				return retryBulkCandidate{}, err
			}
			return retryBulkCandidate{
				ID:       item.ID,
				SourceID: item.SourceID,
				URL:      item.URL,
			}, nil
		},
		func(ctx context.Context, item retryBulkCandidate) error {
			return h.publisher.SendItemCreatedWithReasonE(ctx, item.ID, item.SourceID, item.URL, nil, "retry")
		},
	)

	if result.QueuedCount > 0 {
		if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
			log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
		}
		for _, itemID := range result.queuedItemIDs {
			if err := h.bumpItemDetailVersion(r.Context(), itemID); err != nil {
				log.Printf("item-detail version bump failed item_id=%s err=%v", itemID, err)
			}
		}
	}
	h.invalidateUserCaches(r.Context(), userID)
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, result)
}

func (h *ItemHandler) DeleteBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body retryBulkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	itemIDs := normalizeBulkItemIDs(body.ItemIDs)
	if len(itemIDs) == 0 {
		http.Error(w, "item_ids is required", http.StatusBadRequest)
		return
	}
	if len(itemIDs) > 100 {
		http.Error(w, "too many item_ids", http.StatusBadRequest)
		return
	}

	result := runDeleteBulk(
		r.Context(),
		itemIDs,
		func(ctx context.Context, itemID string) error {
			item, err := h.getItemDetail(ctx, userID, itemID, false)
			if err != nil {
				log.Printf("item delete detail preload failed item_id=%s user_id=%s err=%v", itemID, userID, err)
				item = nil
			}
			if err := h.repo.Delete(ctx, itemID, userID); err != nil {
				return err
			}
			if err := h.bumpItemDetailVersion(ctx, itemID); err != nil {
				log.Printf("item-detail version bump failed item_id=%s err=%v", itemID, err)
			}
			if err := h.publisher.SendItemSearchDeleteE(ctx, itemID); err != nil {
				log.Printf("item-search delete enqueue failed item_id=%s err=%v", itemID, err)
			}
			h.enqueueSearchSuggestionDelete(ctx, userID, item, itemID)
			return nil
		},
	)

	if result.UpdatedCount > 0 {
		if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
			log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
		}
		h.invalidateUserCaches(r.Context(), userID)
	}
	writeJSON(w, result)
}

func buildTriageQueueEntries(resp *model.ReadingPlanResponse) []model.TriageQueueEntry {
	if resp == nil {
		return nil
	}
	consumed := make(map[string]struct{}, len(resp.Items))
	entries := make([]model.TriageQueueEntry, 0, len(resp.Items))
	for _, cluster := range resp.Clusters {
		bundle := model.TriageBundle{
			ID:             cluster.ID,
			Label:          cluster.Label,
			Size:           cluster.Size,
			MaxSimilarity:  cluster.MaxSimilarity,
			Representative: cluster.Representative,
			Items:          cluster.Items,
			Summary:        cluster.Representative.Summary,
			SharedTopics:   triageSharedTopics(cluster.Items),
		}
		for _, item := range cluster.Items {
			consumed[item.ID] = struct{}{}
		}
		entries = append(entries, model.TriageQueueEntry{
			EntryType: "bundle",
			Bundle:    &bundle,
		})
	}
	for _, item := range resp.Items {
		if _, ok := consumed[item.ID]; ok {
			continue
		}
		itemCopy := item
		entries = append(entries, model.TriageQueueEntry{
			EntryType: "item",
			Item:      &itemCopy,
		})
	}
	return entries
}

func triageSharedTopics(items []model.Item) []string {
	if len(items) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, item := range items {
		seen := map[string]struct{}{}
		for _, topic := range item.SummaryTopics {
			topic = strings.TrimSpace(topic)
			if topic == "" {
				continue
			}
			if _, ok := seen[topic]; ok {
				continue
			}
			seen[topic] = struct{}{}
			counts[topic]++
		}
	}
	shared := make([]string, 0, len(counts))
	for topic, count := range counts {
		if count >= 2 {
			shared = append(shared, topic)
		}
	}
	sort.Strings(shared)
	return shared
}

func triageCompletedCount(entries []model.TriageQueueEntry) int {
	completed := 0
	for _, entry := range entries {
		if entry.EntryType == "bundle" && entry.Bundle != nil {
			allRead := len(entry.Bundle.Items) > 0
			for _, item := range entry.Bundle.Items {
				if !item.IsRead {
					allRead = false
					break
				}
			}
			if allRead {
				completed++
			}
			continue
		}
		if entry.Item != nil && entry.Item.IsRead {
			completed++
		}
	}
	return completed
}

func buildTriageQueueParams(q url.Values) (repository.ReadingPlanParams, error) {
	mode := strings.TrimSpace(q.Get("mode"))
	window := q.Get("window")
	if mode == "all" {
		window = "all"
	}
	if window == "" {
		window = "24h"
	}
	size := parseIntOrDefault(q.Get("size"), 20)
	if size < 1 || size > 100 {
		return repository.ReadingPlanParams{}, errors.New("invalid size")
	}
	return repository.ReadingPlanParams{
		Window:          window,
		Size:            size,
		DiversifyTopics: q.Get("diversify_topics") != "false",
		ExcludeRead:     true,
		ExcludeLater:    q.Get("exclude_later") != "false",
	}, nil
}

func NewItemHandler(
	repo *repository.ItemRepo,
	sourceRepo *repository.SourceRepo,
	readingGoalRepo *repository.ReadingGoalRepo,
	streakRepo *repository.ReadingStreakRepo,
	snapshotRepo *repository.BriefingSnapshotRepo,
	prefProfileRepo *repository.PreferenceProfileRepo,
	reviewQueueRepo *repository.ReviewQueueRepo,
	settingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	publisher *service.EventPublisher,
	cipher *service.SecretCipher,
	worker *service.WorkerClient,
	cache service.JSONCache,
	search *service.MeilisearchService,
	keyProvider *service.UserKeyProvider,
) *ItemHandler {
	return &ItemHandler{
		repo:            repo,
		sourceRepo:      sourceRepo,
		readingGoalRepo: readingGoalRepo,
		streakRepo:      streakRepo,
		snapshotRepo:    snapshotRepo,
		prefProfileRepo: prefProfileRepo,
		reviewQueueRepo: reviewQueueRepo,
		settingsRepo:    settingsRepo,
		llmUsageRepo:    llmUsageRepo,
		publisher:       publisher,
		cipher:          cipher,
		worker:          worker,
		cache:           cache,
		search:          search,
		searchItems:     service.NewItemSearchService(search, repo),
		searchSuggest:   service.NewSearchSuggestionService(search),
		detail:          service.NewItemDetailService(repo),
		keyProvider:     keyProvider,
	}
}

func (h *ItemHandler) Navigator(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	itemID := chi.URLParam(r, "id")
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	preview := r.URL.Query().Get("navigator_preview") == "1"

	if h.settingsRepo == nil {
		writeJSON(w, model.ItemNavigatorEnvelope{})
		return
	}
	settings, err := h.settingsRepo.EnsureDefaults(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if settings == nil || !settings.NavigatorEnabled {
		writeJSON(w, model.ItemNavigatorEnvelope{})
		return
	}
	persona := selectBriefingNavigatorPersona(r.Context(), h.cache, userID, settings)
	modelName := resolveBriefingNavigatorModel(settings)
	resolvedModel := ""
	if modelName != nil {
		resolvedModel = strings.TrimSpace(*modelName)
	}
	cacheKey := cacheKeyItemNavigator(userID, itemID, persona, resolvedModel, preview)
	if h.cache != nil && !cacheBust {
		var cached model.ItemNavigatorEnvelope
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			if cached.Navigator != nil && strings.TrimSpace(cached.Navigator.Commentary) != "" {
				writeJSON(w, cached)
				return
			}
		}
	}

	generatedAt := timeutil.NowJST()
	var navigator *model.ItemNavigator
	if preview {
		navigator = h.buildItemNavigatorPreview(r.Context(), userID, itemID, generatedAt, persona)
	} else {
		navigator = h.buildItemNavigator(r.Context(), userID, itemID, generatedAt, persona)
	}
	resp := model.ItemNavigatorEnvelope{Navigator: navigator}
	if h.cache != nil && navigator != nil && strings.TrimSpace(navigator.Commentary) != "" {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, briefingNavigatorCacheTTL); err != nil {
			log.Printf("item navigator cache set failed user_id=%s item_id=%s key=%s err=%v", userID, itemID, cacheKey, err)
		}
	}
	if !preview && navigator != nil && strings.TrimSpace(navigator.Commentary) != "" {
		rememberBriefingNavigatorPersona(r.Context(), h.cache, userID, persona)
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) buildItemNavigator(ctx context.Context, userID, itemID string, generatedAt time.Time, persona string) *model.ItemNavigator {
	if h.detail == nil || h.settingsRepo == nil || h.worker == nil || h.keyProvider == nil {
		return nil
	}
	settings, err := h.settingsRepo.EnsureDefaults(ctx, userID)
	if err != nil {
		log.Printf("item navigator settings user=%s item=%s: %v", userID, itemID, err)
		return nil
	}
	if settings == nil || !settings.NavigatorEnabled {
		return nil
	}
	modelName := resolveBriefingNavigatorModel(settings)
	if modelName == nil {
		return nil
	}
	item, err := h.detail.Get(ctx, itemID, userID)
	if err != nil {
		log.Printf("item navigator detail user=%s item=%s: %v", userID, itemID, err)
		return nil
	}
	if item == nil || item.Status == "deleted" || item.Summary == nil || strings.TrimSpace(item.Summary.Summary) == "" {
		return nil
	}
	facts := []string{}
	if item.Facts != nil {
		for _, fact := range item.Facts.Facts {
			fact = strings.TrimSpace(fact)
			if fact != "" {
				facts = append(facts, fact)
			}
		}
	}
	if len(facts) == 0 {
		return nil
	}

	nk := loadNavigatorKeys(ctx, h.keyProvider, userID, modelName)

	var publishedAt *string
	if item.PublishedAt != nil {
		v := item.PublishedAt.Format(time.RFC3339)
		publishedAt = &v
	}
	workerCtx := service.WithWorkerTraceMetadata(ctx, "item_navigator", &userID, nil, &itemID, nil)
	resp, err := h.worker.GenerateItemNavigatorWithModel(
		workerCtx,
		persona,
		service.ItemNavigatorArticle{
			ItemID:          item.ID,
			Title:           item.Title,
			TranslatedTitle: item.TranslatedTitle,
			SourceTitle:     item.SourceTitle,
			Summary:         strings.TrimSpace(item.Summary.Summary),
			Facts:           facts,
			PublishedAt:     publishedAt,
		},
		nk.anthropicKey,
		nk.googleKey,
		nk.groqKey,
		nk.deepseekKey,
		nk.alibabaKey,
		nk.mistralKey,
		nk.xaiKey,
		nk.zaiKey,
		nk.fireworksKey,
		nk.openAIKey,
		modelName,
	)
	if err != nil {
		log.Printf("item navigator worker user=%s item=%s model=%s: %v", userID, itemID, strings.TrimSpace(*modelName), err)
		return nil
	}
	recordAskLLMUsage(ctx, h.llmUsageRepo, h.cache, "item_navigator", resp.LLM, &userID)
	if strings.TrimSpace(resp.Commentary) == "" {
		return nil
	}
	meta := briefingNavigatorPersonaMeta(persona)
	return &model.ItemNavigator{
		Enabled:        true,
		ItemID:         item.ID,
		Persona:        persona,
		CharacterName:  meta.CharacterName,
		CharacterTitle: meta.CharacterTitle,
		AvatarStyle:    meta.AvatarStyle,
		SpeechStyle:    meta.SpeechStyle,
		Headline:       strings.TrimSpace(resp.Headline),
		Commentary:     strings.TrimSpace(resp.Commentary),
		StanceTags:     resp.StanceTags,
		GeneratedAt:    &generatedAt,
		LLM:            navigatorLLMMeta(resp.LLM),
	}
}

func (h *ItemHandler) buildItemNavigatorPreview(ctx context.Context, userID, itemID string, generatedAt time.Time, persona string) *model.ItemNavigator {
	if h.settingsRepo == nil {
		return nil
	}
	settings, err := h.settingsRepo.EnsureDefaults(ctx, userID)
	if err != nil || settings == nil || !settings.NavigatorEnabled {
		return nil
	}
	meta := briefingNavigatorPersonaMeta(persona)
	return &model.ItemNavigator{
		Enabled:        true,
		ItemID:         itemID,
		Persona:        persona,
		CharacterName:  meta.CharacterName,
		CharacterTitle: meta.CharacterTitle,
		AvatarStyle:    meta.AvatarStyle,
		SpeechStyle:    meta.SpeechStyle,
		Headline:       "見た目確認用の論評プレビュー",
		Commentary:     "ここでは記事詳細の右下アイコンから開く論評オーバーレイの見た目を確認できます。要点の整理だけでなく、どこを面白がるかや、どこを少し警戒するかまで、キャラごとの調子で4〜7文ほど語る想定です。\n\nクリック起点で生成するので、通常はページを開いただけではコストは発生しません。",
		StanceTags:     []string{"preview", persona},
		GeneratedAt:    &generatedAt,
	}
}

func itemsListCacheTTLForSort(sort string) time.Duration {
	switch sort {
	case "score":
		return 2 * time.Minute
	case "personal_score":
		return 5 * time.Minute
	default:
		return time.Minute
	}
}

func (h *ItemHandler) itemsListCacheKey(ctx context.Context, userID, status, sourceID, topic, genre, searchQuery, searchMode string, unreadOnly, readOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) (string, error) {
	version := int64(0)
	if h.cache != nil {
		var err error
		version, err = h.cache.GetVersion(ctx, cacheVersionKeyUserItems(userID))
		if err != nil {
			return "", err
		}
	}
	return cacheKeyItemsListVersioned(userID, version, status, sourceID, topic, genre, searchQuery, searchMode, unreadOnly, readOnly, favoriteOnly, laterOnly, sort, page, pageSize), nil
}

func (h *ItemHandler) bumpUserItemsVersion(ctx context.Context, userID string) error {
	if h.cache == nil || userID == "" {
		return nil
	}
	_, err := h.cache.BumpVersion(ctx, cacheVersionKeyUserItems(userID))
	return err
}

func (h *ItemHandler) itemDetailCacheKey(ctx context.Context, userID, itemID string) (string, error) {
	version := int64(0)
	if h.cache != nil {
		var err error
		version, err = h.cache.GetVersion(ctx, cacheVersionKeyItemDetail(itemID))
		if err != nil {
			return "", err
		}
	}
	return cacheKeyItemDetailVersioned(userID, itemID, version), nil
}

func (h *ItemHandler) bumpItemDetailVersion(ctx context.Context, itemID string) error {
	if h.cache == nil || itemID == "" {
		return nil
	}
	_, err := h.cache.BumpVersion(ctx, cacheVersionKeyItemDetail(itemID))
	return err
}

func (h *ItemHandler) getItemDetail(ctx context.Context, userID, itemID string, cacheBust bool) (*model.ItemDetail, error) {
	cacheKey, cacheKeyErr := h.itemDetailCacheKey(ctx, userID, itemID)
	if cacheKeyErr != nil {
		log.Printf("item-detail cache key failed user_id=%s item_id=%s err=%v", userID, itemID, cacheKeyErr)
	}
	if h.cache != nil && !cacheBust && cacheKeyErr == nil {
		var cached model.ItemDetail
		if ok, err := h.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			incrCacheMetric(ctx, h.cache, userID, "item_detail.hit")
			return &cached, nil
		} else if err != nil {
			incrCacheMetric(ctx, h.cache, userID, "item_detail.error")
			log.Printf("item-detail cache get failed user_id=%s item_id=%s key=%s err=%v", userID, itemID, cacheKey, err)
		}
		incrCacheMetric(ctx, h.cache, userID, "item_detail.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(ctx, h.cache, userID, "item_detail.bypass")
	}

	item, err := h.detail.Get(ctx, itemID, userID)
	if err != nil {
		return nil, err
	}
	if h.cache != nil && item != nil && cacheKeyErr == nil {
		if err := h.cache.SetJSON(ctx, cacheKey, item, itemDetailCacheTTL); err != nil {
			incrCacheMetric(ctx, h.cache, userID, "item_detail.error")
			log.Printf("item-detail cache set failed user_id=%s item_id=%s key=%s err=%v", userID, itemID, cacheKey, err)
		}
	}
	return item, nil
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	var status, sourceID, topic, genre *string
	if v := q.Get("status"); v != "" {
		status = &v
	}
	if v := q.Get("source_id"); v != "" {
		sourceID = &v
	}
	if v := q.Get("topic"); v != "" {
		topic = &v
	}
	if v := q.Get("genre"); v != "" {
		genre = &v
	}
	page := parseIntOrDefault(q.Get("page"), 1)
	pageSize := parseIntOrDefault(q.Get("page_size"), 20)
	if page < 1 || page > 100000 {
		http.Error(w, "invalid page", http.StatusBadRequest)
		return
	}
	if pageSize < 1 || pageSize > 200 {
		http.Error(w, "invalid page_size", http.StatusBadRequest)
		return
	}
	sort := q.Get("sort")
	if sort == "" {
		sort = "newest"
	}
	if sort != "newest" && sort != "score" && sort != "personal_score" {
		http.Error(w, "invalid sort", http.StatusBadRequest)
		return
	}
	unreadOnly := q.Get("unread_only") == "true"
	readOnly := q.Get("read_only") == "true"
	favoriteOnly := q.Get("favorite_only") == "true"
	laterOnly := q.Get("later_only") == "true"
	searchQuery := strings.TrimSpace(q.Get("q"))
	if unreadOnly && readOnly {
		http.Error(w, "unread_only and read_only cannot both be true", http.StatusBadRequest)
		return
	}
	searchMode := strings.TrimSpace(q.Get("search_mode"))
	cacheKey, cacheKeyErr := h.itemsListCacheKey(r.Context(), userID, q.Get("status"), q.Get("source_id"), q.Get("topic"), q.Get("genre"), searchQuery, searchMode, unreadOnly, readOnly, favoriteOnly, laterOnly, sort, page, pageSize)
	cacheBust := q.Get("cache_bust") == "1"
	if cacheKeyErr != nil {
		itemsListCacheCounter.errors.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "items_list.error")
		log.Printf("items-list cache key failed user_id=%s err=%v", userID, cacheKeyErr)
	}
	if h.cache != nil && !cacheBust && cacheKeyErr == nil {
		var cached model.ItemListResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			itemsListCacheCounter.hits.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.hit")
			writeJSON(w, &cached)
			return
		} else if err != nil {
			itemsListCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.error")
			log.Printf("items-list cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		itemsListCacheCounter.misses.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "items_list.miss")
	} else if cacheBust {
		itemsListCacheCounter.bypass.Add(1)
		if h.cache != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.bypass")
		}
	}

	var queryPtr *string
	if searchQuery != "" {
		queryPtr = &searchQuery
	}
	var resp *model.ItemListResponse
	var err error
	if queryPtr != nil && h.searchItems != nil {
		resp, err = h.searchItems.Search(r.Context(), service.ItemSearchQuery{
			UserID:       userID,
			Query:        searchQuery,
			SearchMode:   searchMode,
			Status:       status,
			SourceID:     sourceID,
			Topic:        topic,
			Genre:        genre,
			UnreadOnly:   unreadOnly,
			ReadOnly:     readOnly,
			FavoriteOnly: favoriteOnly,
			LaterOnly:    laterOnly,
			Page:         page,
			PageSize:     pageSize,
		})
		if err != nil {
			log.Printf("items search unavailable user_id=%s err=%v", userID, err)
			resp = &model.ItemListResponse{
				Items:             []model.Item{},
				Page:              page,
				PageSize:          pageSize,
				Total:             0,
				HasNext:           false,
				Sort:              "relevance",
				Status:            status,
				SourceID:          sourceID,
				SearchUnavailable: true,
			}
			mode := service.NormalizeSearchMode(searchMode)
			resp.SearchMode = &mode
		}
	} else {
		resp, err = h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
			Status:       status,
			SourceID:     sourceID,
			Topic:        topic,
			Genre:        genre,
			Query:        queryPtr,
			UnreadOnly:   unreadOnly,
			ReadOnly:     readOnly,
			FavoriteOnly: favoriteOnly,
			LaterOnly:    laterOnly,
			Sort:         sort,
			Page:         page,
			PageSize:     pageSize,
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
	}
	if resp != nil && sort == "personal_score" {
		missingPersonalScores := make([]string, 0, len(resp.Items))
		for _, item := range resp.Items {
			if item.PersonalScore == nil {
				missingPersonalScores = append(missingPersonalScores, item.ID)
			}
		}
		if len(missingPersonalScores) > 0 {
			h.applyPersonalScoreSort(r.Context(), userID, resp)
			if persistErr := h.repo.PersistPersonalScores(r.Context(), userID, missingPersonalScores); persistErr != nil {
				log.Printf("personal_score persist on list failed user_id=%s count=%d err=%v", userID, len(missingPersonalScores), persistErr)
			}
		}
	}
	if h.cache != nil && resp != nil && cacheKeyErr == nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, itemsListCacheTTLForSort(sort)); err != nil {
			itemsListCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "items_list.error")
			log.Printf("items-list cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) applyPersonalScoreSort(ctx context.Context, userID string, resp *model.ItemListResponse) {
	if h.prefProfileRepo == nil {
		return
	}
	profile, err := h.prefProfileRepo.GetProfile(ctx, userID)
	if err != nil || profile == nil || profile.FeedbackCount < 10 {
		return
	}

	itemIDs := make([]string, len(resp.Items))
	for i, it := range resp.Items {
		itemIDs[i] = it.ID
	}
	embeddings, err := h.repo.LoadItemEmbeddingsByID(ctx, itemIDs)
	if err != nil {
		log.Printf("personal_score: load embeddings failed user_id=%s err=%v", userID, err)
		embeddings = nil
	}

	for i := range resp.Items {
		it := &resp.Items[i]
		input := repository.PersonalScoreInput{
			SummaryScore:   it.SummaryScore,
			ScoreBreakdown: it.SummaryScoreBreakdown,
			Topics:         it.SummaryTopics,
			SourceID:       it.SourceID,
			PublishedAt:    it.PublishedAt,
			FetchedAt:      it.FetchedAt,
			CreatedAt:      it.CreatedAt,
		}
		if embeddings != nil {
			input.Embedding = embeddings[it.ID]
		}
		score, reason := repository.CalcPersonalScore(input, profile)
		it.PersonalScore = &score
		it.PersonalScoreReason = &reason
	}

	sort.SliceStable(resp.Items, func(i, j int) bool {
		si := 0.0
		sj := 0.0
		if resp.Items[i].PersonalScore != nil {
			si = *resp.Items[i].PersonalScore
		}
		if resp.Items[j].PersonalScore != nil {
			sj = *resp.Items[j].PersonalScore
		}
		return si > sj
	})
}

func (h *ItemHandler) ExportFavoritesMarkdown(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 30)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
	if days < 0 || days > 3650 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	if limit < 1 || limit > 200 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}

	items, err := h.repo.FavoriteExportItems(r.Context(), userID, days, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	now := timeutil.NowJST()
	rangeLabel := "all favorites"
	if days > 0 {
		rangeLabel = fmt.Sprintf("last %d days", days)
	}
	filenameDate := now.Format("2006-01-02")
	filename := fmt.Sprintf("sifto-favorites-%s.md", filenameDate)

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write([]byte(buildFavoritesMarkdown(items, now, rangeLabel)))
}

func buildFavoritesMarkdown(items []model.FavoriteExportItem, now time.Time, rangeLabel string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Sifto favorites\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", now.Format("2006-01-02 15:04 JST"))
	fmt.Fprintf(&b, "Range: %s\n\n", rangeLabel)
	fmt.Fprintf(&b, "Items: %d\n\n", len(items))
	for i, item := range items {
		title := pickFavoriteExportTitle(item)
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, title)
		fmt.Fprintf(&b, "- URL: %s\n", item.URL)
		fmt.Fprintf(&b, "- Favorited: %s\n", item.FavoritedAt.In(timeutil.JST).Format("2006-01-02 15:04 JST"))
		if item.PublishedAt != nil {
			fmt.Fprintf(&b, "- Published: %s\n", item.PublishedAt.In(timeutil.JST).Format("2006-01-02 15:04 JST"))
		}
		if item.SourceTitle != nil && strings.TrimSpace(*item.SourceTitle) != "" {
			fmt.Fprintf(&b, "- Source: %s\n", strings.TrimSpace(*item.SourceTitle))
		}
		if item.SummaryScore != nil {
			fmt.Fprintf(&b, "- Score: %.2f\n", *item.SummaryScore)
		}
		if len(item.Topics) > 0 {
			fmt.Fprintf(&b, "- Topics: %s\n", strings.Join(item.Topics, ", "))
		}
		b.WriteString("\n")
		if item.Summary != nil && strings.TrimSpace(*item.Summary) != "" {
			b.WriteString(strings.TrimSpace(*item.Summary))
			b.WriteString("\n\n")
		} else {
			b.WriteString("Summary: (not available)\n\n")
		}
		if item.Note != nil && strings.TrimSpace(item.Note.Content) != "" {
			b.WriteString("### Personal Note\n\n")
			b.WriteString(strings.TrimSpace(item.Note.Content))
			b.WriteString("\n\n")
			if len(item.Note.Tags) > 0 {
				for _, tag := range item.Note.Tags {
					tag = strings.TrimSpace(tag)
					if tag == "" {
						continue
					}
					fmt.Fprintf(&b, "- Tag: %s\n", tag)
				}
				b.WriteString("\n")
			}
		}
		if len(item.Highlights) > 0 {
			b.WriteString("### Highlights\n\n")
			for _, highlight := range item.Highlights {
				quote := strings.TrimSpace(highlight.QuoteText)
				if quote == "" {
					continue
				}
				fmt.Fprintf(&b, "- %s", quote)
				meta := make([]string, 0, 2)
				if section := strings.TrimSpace(highlight.Section); section != "" {
					meta = append(meta, "section: "+section)
				}
				if anchor := strings.TrimSpace(highlight.AnchorText); anchor != "" {
					meta = append(meta, "anchor: "+anchor)
				}
				if len(meta) > 0 {
					fmt.Fprintf(&b, " (%s)", strings.Join(meta, ", "))
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func pickFavoriteExportTitle(item model.FavoriteExportItem) string {
	if item.TranslatedTitle != nil && strings.TrimSpace(*item.TranslatedTitle) != "" {
		return strings.TrimSpace(*item.TranslatedTitle)
	}
	if item.Title != nil && strings.TrimSpace(*item.Title) != "" {
		return strings.TrimSpace(*item.Title)
	}
	return item.URL
}

func (h *ItemHandler) Stats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	resp, err := h.repo.Stats(r.Context(), userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) UXMetrics(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 7)
	if days < 1 || days > 90 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	today := timeutil.StartOfDayJST(timeutil.NowJST())
	todayStr := today.Format("2006-01-02")
	fromStr := today.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	todayNew, err := h.repo.CountNewOnDateJST(r.Context(), userID, todayStr)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	todayRead, err := h.repo.CountReadOnDateJST(r.Context(), userID, todayStr)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	periodRead, activeDays, err := h.repo.ReadActivityInRangeJST(r.Context(), userID, fromStr, todayStr)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	var todayRate *float64
	if todayNew > 0 {
		v := float64(todayRead) / float64(todayNew)
		todayRate = &v
	}
	avgReads := float64(periodRead) / float64(days)

	streak := 0
	if h.streakRepo != nil {
		if _, streakDays, _, err := h.streakRepo.GetByUserAndDate(r.Context(), userID, todayStr); err == nil {
			streak = streakDays
		} else {
			yesterdayStr := today.AddDate(0, 0, -1).Format("2006-01-02")
			if _, streakDays, _, err := h.streakRepo.GetByUserAndDate(r.Context(), userID, yesterdayStr); err == nil {
				streak = streakDays
			}
		}
	}

	writeJSON(w, &model.ItemUXMetricsResponse{
		Days:                     days,
		TodayDate:                todayStr,
		TodayNewItems:            todayNew,
		TodayReadItems:           todayRead,
		TodayConsumptionRate:     todayRate,
		PeriodReadItems:          periodRead,
		PeriodActiveReadDays:     activeDays,
		PeriodAverageReadsPerDay: avgReads,
		CurrentStreakDays:        streak,
	})
}

func (h *ItemHandler) invalidateUserCaches(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	if h.cache != nil {
		for _, prefix := range cacheUserInvalidatePrefixes(userID) {
			if _, err := h.cache.DeleteByPrefix(ctx, prefix, 5000); err != nil {
				log.Printf("cache invalidate failed user_id=%s prefix=%s err=%v", userID, prefix, err)
			}
		}
	}
	if h.snapshotRepo != nil {
		today := timeutil.StartOfDayJST(timeutil.NowJST()).Format("2006-01-02")
		if err := h.snapshotRepo.MarkStale(ctx, userID, today); err != nil {
			log.Printf("briefing snapshot stale failed user_id=%s date=%s err=%v", userID, today, err)
		}
	}
}

func (h *ItemHandler) refreshPreferenceProfileAsync(userID, itemID string) {
	if userID == "" || itemID == "" || h.prefProfileRepo == nil {
		return
	}
	safeGo(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		profile, profileErr := h.prefProfileRepo.BuildProfileForUser(ctx, userID)
		if profileErr != nil {
			log.Printf("preference profile rebuild failed user_id=%s err=%v", userID, profileErr)
			return
		}
		if upsertErr := h.prefProfileRepo.UpsertProfile(ctx, profile); upsertErr != nil {
			log.Printf("preference profile upsert failed user_id=%s err=%v", userID, upsertErr)
			return
		}
		if persistErr := h.repo.PersistPersonalScores(ctx, userID, []string{itemID}); persistErr != nil {
			log.Printf("personal score persist failed user_id=%s item_id=%s err=%v", userID, itemID, persistErr)
		}
	})
}

func (h *ItemHandler) TopicTrends(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 8)
	if limit < 1 || limit > 50 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.TopicTrends(r.Context(), userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, topicTrendsResponse{Items: rows, Limit: limit})
}

func (h *ItemHandler) TopicPulse(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	days := parseIntOrDefault(r.URL.Query().Get("days"), 7)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 12)
	if days < 1 || days > 30 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}
	if limit < 1 || limit > 50 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	rows, err := h.repo.TopicPulse(r.Context(), userID, days, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, topicPulseResponse{Days: days, Limit: limit, Items: rows})
}

func (h *ItemHandler) ReadingPlan(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	window := q.Get("window")
	if window == "" {
		window = "24h"
	}
	size := parseIntOrDefault(q.Get("size"), 15)
	if size < 1 || size > 100 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	diversify := q.Get("diversify_topics") != "false"
	excludeRead := q.Get("exclude_read") != "false"
	params := repository.ReadingPlanParams{
		Window:          window,
		Size:            size,
		DiversifyTopics: diversify,
		ExcludeRead:     excludeRead,
		ExcludeLater:    q.Get("exclude_later") == "true",
	}
	cacheKey := cacheKeyReadingPlan(userID, params.Window, params.Size, params.DiversifyTopics, params.ExcludeRead, params.ExcludeLater)
	cacheBust := q.Get("cache_bust") == "1"
	resp, err := cachedFetchWithOpts(r.Context(), h.cache, cacheKey, 120*time.Second, func() (*model.ReadingPlanResponse, error) {
		return h.repo.ReadingPlan(r.Context(), userID, params)
	}, cacheFetchOptions{
		cacheBust:    cacheBust,
		metricPrefix: "reading_plan",
		userID:       userID,
		counter:      &readingPlanCacheCounter,
		logKeyPrefix: "reading-plan",
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) FocusQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	window := q.Get("window")
	if window == "" {
		window = "24h"
	}
	size := parseIntOrDefault(q.Get("size"), 20)
	if size < 1 || size > 100 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	params := repository.ReadingPlanParams{
		Window:          window,
		Size:            size,
		DiversifyTopics: q.Get("diversify_topics") != "false",
		ExcludeRead:     false,
		ExcludeLater:    q.Get("exclude_later") != "false",
	}
	cacheKey := cacheKeyFocusQueue(userID, params.Window, params.Size, params.DiversifyTopics, params.ExcludeLater)
	cacheBust := q.Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.error")
			log.Printf("focus-queue cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.bypass")
	}

	resp, err := h.repo.ReadingPlan(r.Context(), userID, params)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if resp == nil {
		out := focusQueueResponse{
			Items:      []model.Item{},
			Size:       size,
			Window:     window,
			Completed:  0,
			Remaining:  0,
			Total:      0,
			SourcePool: 0,
		}
		if h.cache != nil {
			if err := h.cache.SetJSON(r.Context(), cacheKey, out, focusQueueCacheTTL); err != nil {
				incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.error")
				log.Printf("focus-queue cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
			}
		}
		writeJSON(w, out)
		return
	}
	items := resp.Items
	completed := 0
	for _, it := range items {
		if it.IsRead {
			completed++
		}
	}
	out := focusQueueResponse{
		Items:           items,
		Size:            size,
		Window:          resp.Window,
		Completed:       completed,
		Remaining:       len(items) - completed,
		Total:           len(items),
		SourcePool:      resp.SourcePoolCount,
		DiversifyTopics: resp.DiversifyTopics,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, focusQueueCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "focus_queue.error")
			log.Printf("focus-queue cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func (h *ItemHandler) TriageQueue(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	params, err := buildTriageQueueParams(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cacheKey := cacheKeyTriageQueue(userID, params.Window, params.Size, params.DiversifyTopics, params.ExcludeLater)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached model.TriageQueueResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_queue.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_queue.error")
			log.Printf("triage-queue cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "triage_queue.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "triage_queue.bypass")
	}

	out := model.TriageQueueResponse{
		Entries:         nil,
		Window:          params.Window,
		Size:            params.Size,
		Completed:       0,
		Remaining:       0,
		Total:           0,
		UnderlyingItems: 0,
		BundleCount:     0,
		SourcePool:      0,
		DiversifyTopics: params.DiversifyTopics,
	}
	if params.Window == "all" {
		items := make([]model.Item, 0, 200)
		page := 1
		for page <= 100 {
			resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
				Page:         page,
				PageSize:     200,
				Sort:         "newest",
				UnreadOnly:   true,
				FavoriteOnly: false,
				LaterOnly:    false,
			})
			if err != nil {
				writeRepoError(w, err)
				return
			}
			if resp == nil || len(resp.Items) == 0 {
				break
			}
			items = append(items, resp.Items...)
			if !resp.HasNext {
				break
			}
			page++
		}
		triageClusters, err := h.repo.TriageClustersByEmbeddings(r.Context(), items, nil)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		allResp := &model.ReadingPlanResponse{
			Items:           items,
			Window:          "all",
			Size:            len(items),
			DiversifyTopics: false,
			ExcludeRead:     true,
			SourcePoolCount: len(items),
			Clusters:        triageClusters,
		}
		out.Entries = buildTriageQueueEntries(allResp)
		out.Window = allResp.Window
		out.Size = allResp.Size
		out.Total = len(out.Entries)
		out.Completed = triageCompletedCount(out.Entries)
		out.Remaining = len(out.Entries) - out.Completed
		out.UnderlyingItems = len(allResp.Items)
		out.BundleCount = len(allResp.Clusters)
		out.SourcePool = allResp.SourcePoolCount
		out.DiversifyTopics = allResp.DiversifyTopics
	} else {
		resp, err := h.repo.ReadingPlan(r.Context(), userID, params)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if resp != nil {
			triageClusters, err := h.repo.TriageClustersByEmbeddings(r.Context(), resp.Items, nil)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			resp.Clusters = triageClusters
			out.Entries = buildTriageQueueEntries(resp)
			out.Window = resp.Window
			out.Size = resp.Size
			out.Total = len(out.Entries)
			out.Completed = triageCompletedCount(out.Entries)
			out.Remaining = len(out.Entries) - out.Completed
			out.UnderlyingItems = len(resp.Items)
			out.BundleCount = len(resp.Clusters)
			out.SourcePool = resp.SourcePoolCount
			out.DiversifyTopics = resp.DiversifyTopics
		}
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, focusQueueCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_queue.error")
			log.Printf("triage-queue cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func (h *ItemHandler) TriageAll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	cacheBust := q.Get("cache_bust") == "1"
	cacheKey := cacheKeyTriageAll(userID)
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_all.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_all.error")
			log.Printf("triage-all cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "triage_all.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "triage_all.bypass")
	}

	items := make([]model.Item, 0, 200)
	page := 1
	for page <= 100 {
		resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
			Page:         page,
			PageSize:     200,
			Sort:         "newest",
			UnreadOnly:   true,
			FavoriteOnly: false,
			LaterOnly:    false,
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if resp == nil || len(resp.Items) == 0 {
			break
		}
		items = append(items, resp.Items...)
		if !resp.HasNext {
			break
		}
		page++
	}
	out := focusQueueResponse{
		Items:           items,
		Size:            len(items),
		Window:          "all",
		Completed:       0,
		Remaining:       len(items),
		Total:           len(items),
		SourcePool:      0,
		DiversifyTopics: false,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, triageAllCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "triage_all.error")
			log.Printf("triage-all cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func (h *ItemHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.getItemDetail(r.Context(), userID, id, r.URL.Query().Get("cache_bust") == "1")
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.applyPersonalizationToDetail(r.Context(), userID, item)
	writeJSON(w, item)
}

func (h *ItemHandler) UpdateGenre(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	var body struct {
		UserGenre *string `json:"user_genre"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateUserGenre(r.Context(), userID, id, body.UserGenre); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if h.publisher != nil {
		if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
			log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
		}
	}
	h.invalidateUserCaches(r.Context(), userID)
	item, err := h.getItemDetail(r.Context(), userID, id, true)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	resp := struct {
		ItemID    string  `json:"item_id"`
		Genre     string  `json:"genre,omitempty"`
		UserGenre *string `json:"user_genre,omitempty"`
	}{
		ItemID: id,
	}
	if item != nil {
		resp.Genre = item.Genre
		resp.UserGenre = item.UserGenre
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) SearchSuggestions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if h.searchSuggest == nil {
		http.Error(w, "search suggestions unavailable", http.StatusServiceUnavailable)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 10)
	if limit < 1 || limit > 10 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}

	resp, err := h.searchSuggest.Search(r.Context(), service.SearchSuggestionQuery{
		UserID: userID,
		Query:  query,
		Limit:  limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, resp)
}

func (h *ItemHandler) applyPersonalizationToDetail(ctx context.Context, userID string, item *model.ItemDetail) {
	if item == nil || h.prefProfileRepo == nil {
		return
	}

	var profile *model.UserPreferenceProfile
	nextProfile, err := h.prefProfileRepo.GetProfile(ctx, userID)
	if err != nil && err != repository.ErrNotFound {
		log.Printf("item detail preference profile load failed user_id=%s item_id=%s err=%v", userID, item.ID, err)
	} else {
		profile = nextProfile
	}

	input := repository.PersonalScoreInput{
		SummaryScore:   item.SummaryScore,
		ScoreBreakdown: item.SummaryScoreBreakdown,
		Topics:         item.SummaryTopics,
		SourceID:       item.SourceID,
		PublishedAt:    item.PublishedAt,
		FetchedAt:      item.FetchedAt,
		CreatedAt:      item.CreatedAt,
	}
	if h.repo != nil {
		if embByID, embErr := h.repo.LoadItemEmbeddingsByID(ctx, []string{item.ID}); embErr == nil {
			input.Embedding = embByID[item.ID]
		} else {
			log.Printf("item detail embedding load failed item_id=%s err=%v", item.ID, embErr)
		}
	}

	result := repository.CalcPersonalScoreDetailed(input, profile)
	item.PersonalScore = &result.Score
	item.PersonalScoreReason = &result.Reason
	item.PersonalScoreBreakdown = result.Breakdown
}

func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.getItemDetail(r.Context(), userID, id, false)
	if err != nil {
		log.Printf("item delete detail preload failed item_id=%s user_id=%s err=%v", id, userID, err)
		item = nil
	}
	if err := h.repo.Delete(r.Context(), id, userID); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchDeleteE(r.Context(), id); err != nil {
		log.Printf("item-search delete enqueue failed item_id=%s err=%v", id, err)
	}
	h.enqueueSearchSuggestionDelete(r.Context(), userID, item, id)
	h.invalidateUserCaches(r.Context(), userID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ItemHandler) Restore(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.Restore(r.Context(), id, userID); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
		log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	item, err := h.getItemDetail(r.Context(), userID, id, true)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	h.enqueueSearchSuggestionUpsert(r.Context(), userID, item, id)
	writeJSON(w, item)
}

func (h *ItemHandler) enqueueSearchSuggestionUpsert(ctx context.Context, userID string, item *model.ItemDetail, itemID string) {
	if h.publisher == nil {
		return
	}
	if err := h.publisher.SendSearchSuggestionArticleUpsertE(ctx, itemID); err != nil {
		log.Printf("search suggestion article upsert enqueue failed item_id=%s err=%v", itemID, err)
	}
	if item != nil && strings.TrimSpace(item.SourceID) != "" {
		if err := h.publisher.SendSearchSuggestionSourceUpsertE(ctx, item.SourceID); err != nil {
			log.Printf("search suggestion source upsert enqueue failed item_id=%s source_id=%s err=%v", itemID, item.SourceID, err)
		}
	}
	if strings.TrimSpace(userID) != "" {
		if err := h.publisher.SendSearchSuggestionTopicsRefreshE(ctx, userID); err != nil {
			log.Printf("search suggestion topics refresh enqueue failed item_id=%s user_id=%s err=%v", itemID, userID, err)
		}
	}
}

func (h *ItemHandler) enqueueSearchSuggestionDelete(ctx context.Context, userID string, item *model.ItemDetail, itemID string) {
	if h.publisher == nil {
		return
	}
	if err := h.publisher.SendSearchSuggestionArticleDeleteE(ctx, itemID); err != nil {
		log.Printf("search suggestion article delete enqueue failed item_id=%s err=%v", itemID, err)
	}
	if item != nil && strings.TrimSpace(item.SourceID) != "" {
		if err := h.publisher.SendSearchSuggestionSourceUpsertE(ctx, item.SourceID); err != nil {
			log.Printf("search suggestion source upsert enqueue failed item_id=%s source_id=%s err=%v", itemID, item.SourceID, err)
		}
	}
	if strings.TrimSpace(userID) != "" {
		if err := h.publisher.SendSearchSuggestionTopicsRefreshE(ctx, userID); err != nil {
			log.Printf("search suggestion topics refresh enqueue failed item_id=%s user_id=%s err=%v", itemID, userID, err)
		}
	}
}

func (h *ItemHandler) Related(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 6)
	if limit < 1 || limit > 20 {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	cacheKey := cacheKeyRelated(userID, id, limit)
	cacheBust := r.URL.Query().Get("cache_bust") == "1"
	if h.cache != nil && !cacheBust {
		var cached map[string]any
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			incrCacheMetric(r.Context(), h.cache, userID, "related.hit")
			writeJSON(w, cached)
			return
		} else if err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "related.error")
			log.Printf("related cache get failed user_id=%s item_id=%s key=%s err=%v", userID, id, cacheKey, err)
		}
		incrCacheMetric(r.Context(), h.cache, userID, "related.miss")
	} else if cacheBust && h.cache != nil {
		incrCacheMetric(r.Context(), h.cache, userID, "related.bypass")
	}

	var targetTopics []string
	if detail, err := h.getItemDetail(r.Context(), userID, id, false); err == nil && detail != nil && detail.Summary != nil {
		targetTopics = detail.Summary.Topics
	}
	items, err := h.repo.ListRelated(r.Context(), id, userID, limit)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	items = rerankAndFilterRelated(items, targetTopics, limit)
	annotateRelatedReasons(items, targetTopics)
	clusters := clusterRelatedItems(items)
	out := relatedItemsResponse{
		Items:    items,
		Clusters: clusters,
		Limit:    limit,
		ItemID:   id,
	}
	if h.cache != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, out, relatedItemsCacheTTL); err != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "related.error")
			log.Printf("related cache set failed user_id=%s item_id=%s key=%s err=%v", userID, id, cacheKey, err)
		}
	}
	writeJSON(w, out)
}

func rerankAndFilterRelated(items []model.RelatedItem, targetTopics []string, limit int) []model.RelatedItem {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	targetSet := map[string]struct{}{}
	for _, t := range targetTopics {
		v := strings.TrimSpace(t)
		if v == "" {
			continue
		}
		targetSet[v] = struct{}{}
	}
	type scoredItem struct {
		item    model.RelatedItem
		score   float64
		overlap int
	}
	scored := make([]scoredItem, 0, len(items))
	for _, it := range items {
		overlap := 0
		if len(targetSet) > 0 {
			for _, topic := range it.Topics {
				if _, ok := targetSet[strings.TrimSpace(topic)]; ok {
					overlap++
				}
			}
		}
		// Hard filter to cut obvious noise while avoiding "no related items".
		if overlap == 0 && it.Similarity < 0.58 {
			continue
		}
		if overlap > 0 && it.Similarity < 0.42 {
			continue
		}
		overlapBoost := 0.0
		if overlap > 0 {
			overlapBoost = float64(overlap)
			if overlapBoost > 3 {
				overlapBoost = 3
			}
			overlapBoost *= 0.06
		}
		score := it.Similarity + overlapBoost
		scored = append(scored, scoredItem{item: it, score: score, overlap: overlap})
	}
	if len(scored) == 0 {
		// Fallback 1: keep reasonably high-similarity items.
		for _, it := range items {
			if it.Similarity >= 0.62 {
				scored = append(scored, scoredItem{item: it, score: it.Similarity, overlap: 0})
			}
		}
	}
	if len(scored) == 0 {
		// Fallback 2: at least return stronger half of candidates.
		for _, it := range items {
			if it.Similarity >= 0.50 {
				scored = append(scored, scoredItem{item: it, score: it.Similarity, overlap: 0})
			}
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].overlap != scored[j].overlap {
			return scored[i].overlap > scored[j].overlap
		}
		if scored[i].item.Similarity != scored[j].item.Similarity {
			return scored[i].item.Similarity > scored[j].item.Similarity
		}
		return scored[i].item.CreatedAt.After(scored[j].item.CreatedAt)
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]model.RelatedItem, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.item)
	}
	return out
}

func annotateRelatedReasons(items []model.RelatedItem, targetTopics []string) {
	targetSet := map[string]struct{}{}
	for _, t := range targetTopics {
		v := strings.TrimSpace(t)
		if v == "" {
			continue
		}
		targetSet[v] = struct{}{}
	}
	for i := range items {
		var shared []string
		for _, t := range items[i].Topics {
			if _, ok := targetSet[t]; ok {
				shared = append(shared, t)
				if len(shared) >= 3 {
					break
				}
			}
		}
		items[i].ReasonTopics = shared
		if len(shared) > 0 {
			reason := fmt.Sprintf("shared topics: %s", strings.Join(shared, ", "))
			items[i].Reason = &reason
			continue
		}
		switch {
		case items[i].Similarity >= 0.8:
			reason := "very high semantic similarity"
			items[i].Reason = &reason
		case items[i].Similarity >= 0.65:
			reason := "high semantic similarity"
			items[i].Reason = &reason
		default:
			reason := "semantic similarity match"
			items[i].Reason = &reason
		}
	}
}

type relatedClusterResponse struct {
	ID             string              `json:"id"`
	Label          string              `json:"label"`
	Size           int                 `json:"size"`
	MaxSimilarity  float64             `json:"max_similarity"`
	Representative model.RelatedItem   `json:"representative"`
	Items          []model.RelatedItem `json:"items"`
}

func clusterRelatedItems(items []model.RelatedItem) []relatedClusterResponse {
	if len(items) == 0 {
		return nil
	}
	remaining := make([]model.RelatedItem, len(items))
	copy(remaining, items)
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Similarity != remaining[j].Similarity {
			return remaining[i].Similarity > remaining[j].Similarity
		}
		return remaining[i].CreatedAt.After(remaining[j].CreatedAt)
	})

	used := make([]bool, len(remaining))
	clusters := make([]relatedClusterResponse, 0, len(remaining))
	for i := range remaining {
		if used[i] {
			continue
		}
		seed := remaining[i]
		used[i] = true
		members := []model.RelatedItem{seed}
		maxSim := seed.Similarity
		seedTopicSet := map[string]struct{}{}
		for _, t := range seed.Topics {
			if t != "" {
				seedTopicSet[t] = struct{}{}
			}
		}
		for j := i + 1; j < len(remaining); j++ {
			if used[j] {
				continue
			}
			cand := remaining[j]
			if shouldClusterRelated(seed, seedTopicSet, cand) {
				used[j] = true
				members = append(members, cand)
				if cand.Similarity > maxSim {
					maxSim = cand.Similarity
				}
			}
		}
		sort.SliceStable(members, func(a, b int) bool {
			if members[a].Similarity != members[b].Similarity {
				return members[a].Similarity > members[b].Similarity
			}
			return members[a].CreatedAt.After(members[b].CreatedAt)
		})
		label := clusterLabel(members[0])
		clusters = append(clusters, relatedClusterResponse{
			ID:             members[0].ID,
			Label:          label,
			Size:           len(members),
			MaxSimilarity:  maxSim,
			Representative: members[0],
			Items:          members,
		})
	}
	return clusters
}

func shouldClusterRelated(seed model.RelatedItem, seedTopics map[string]struct{}, cand model.RelatedItem) bool {
	// Strong similarity alone groups items.
	if cand.Similarity >= 0.78 {
		return true
	}
	// Otherwise require moderate similarity + topic overlap.
	if cand.Similarity < 0.58 {
		return false
	}
	if len(seedTopics) == 0 || len(cand.Topics) == 0 {
		return false
	}
	for _, t := range cand.Topics {
		if _, ok := seedTopics[t]; ok {
			return true
		}
	}
	return false
}

func clusterLabel(it model.RelatedItem) string {
	if len(it.Topics) > 0 && it.Topics[0] != "" {
		return it.Topics[0]
	}
	if it.Title != nil && *it.Title != "" {
		return *it.Title
	}
	return "Related"
}

func (h *ItemHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	inserted, err := h.repo.MarkRead(r.Context(), userID, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if inserted && h.streakRepo != nil {
		_ = h.streakRepo.IncrementRead(r.Context(), userID, timeutil.NowJST(), 3)
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
		log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, itemToggleResponse{ItemID: id, IsRead: true})
}

func (h *ItemHandler) MarkUnread(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkUnread(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
		log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, itemToggleResponse{ItemID: id, IsRead: false})
}

func (h *ItemHandler) MarkReadBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		ItemIDs       []string `json:"item_ids"`
		Status        *string  `json:"status"`
		SourceID      *string  `json:"source_id"`
		Topic         *string  `json:"topic"`
		UnreadOnly    bool     `json:"unread_only"`
		ReadOnly      bool     `json:"read_only"`
		FavoriteOnly  bool     `json:"favorite_only"`
		LaterOnly     bool     `json:"later_only"`
		OlderThanDays *int     `json:"older_than_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if len(body.ItemIDs) > 0 {
		if len(body.ItemIDs) > 100 {
			http.Error(w, "too many item_ids", http.StatusBadRequest)
			return
		}
		updated, err := h.repo.MarkReadBulkByIDs(r.Context(), userID, body.ItemIDs)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
			log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
		}
		h.invalidateUserCaches(r.Context(), userID)
		writeJSON(w, bulkStatusResponse{Status: "ok", UpdatedCount: updated})
		return
	}
	if body.UnreadOnly && body.ReadOnly {
		http.Error(w, "unread_only and read_only cannot both be true", http.StatusBadRequest)
		return
	}
	updated, err := h.repo.MarkReadBulk(r.Context(), userID, repository.BulkMarkReadParams{
		Status:        body.Status,
		SourceID:      body.SourceID,
		Topic:         body.Topic,
		UnreadOnly:    body.UnreadOnly,
		ReadOnly:      body.ReadOnly,
		FavoriteOnly:  body.FavoriteOnly,
		LaterOnly:     body.LaterOnly,
		OlderThanDays: body.OlderThanDays,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, bulkStatusResponse{Status: "ok", UpdatedCount: updated})
}

func (h *ItemHandler) MarkLater(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkLater(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
		log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, itemLaterResponse{ItemID: id, IsLater: true})
}

func (h *ItemHandler) UnmarkLater(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.UnmarkLater(r.Context(), userID, id); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
		log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, itemLaterResponse{ItemID: id, IsLater: false})
}

func (h *ItemHandler) MarkLaterBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if len(body.ItemIDs) == 0 {
		http.Error(w, "item_ids is required", http.StatusBadRequest)
		return
	}
	if len(body.ItemIDs) > 100 {
		http.Error(w, "too many item_ids", http.StatusBadRequest)
		return
	}
	updated, err := h.repo.MarkLaterBulk(r.Context(), userID, body.ItemIDs)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	for _, itemID := range body.ItemIDs {
		if err := h.bumpItemDetailVersion(r.Context(), itemID); err != nil {
			log.Printf("item-detail version bump failed item_id=%s err=%v", itemID, err)
		}
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, bulkStatusResponse{Status: "ok", UpdatedCount: updated})
}

func (h *ItemHandler) SetFeedback(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	var body struct {
		Rating     int  `json:"rating"`
		IsFavorite bool `json:"is_favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Rating < -1 || body.Rating > 1 {
		http.Error(w, "invalid rating", http.StatusBadRequest)
		return
	}
	fb, err := h.repo.UpsertFeedback(r.Context(), userID, id, body.Rating, body.IsFavorite)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
	if err := h.publisher.SendItemSearchUpsertE(r.Context(), id); err != nil {
		log.Printf("item-search upsert enqueue failed item_id=%s err=%v", id, err)
	}
	if h.reviewQueueRepo != nil && body.IsFavorite {
		_ = h.reviewQueueRepo.EnqueueDefault(r.Context(), userID, id, "favorite", time.Now())
	}
	h.invalidateUserCaches(r.Context(), userID)
	h.refreshPreferenceProfileAsync(userID, id)
	writeJSON(w, fb)
}

func (h *ItemHandler) Retry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.repo.ResetForExtractRetry(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}
	if err := h.publisher.SendItemCreatedWithReasonE(r.Context(), item.ID, item.SourceID, item.URL, nil, "retry"); err != nil {
		http.Error(w, "failed to enqueue retry", http.StatusBadGateway)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), item.ID); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", item.ID, err)
	}
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, retryItemResponse{Status: "queued", ItemID: item.ID})
}

func (h *ItemHandler) RetryFromFacts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.repo.ResetForFactsRetry(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			http.Error(w, "item cannot be retried from facts", http.StatusConflict)
			return
		}
		writeRepoError(w, err)
		return
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}
	if err := h.publisher.SendItemCreatedWithReasonE(r.Context(), item.ID, item.SourceID, item.URL, nil, "retry_from_facts"); err != nil {
		http.Error(w, "failed to enqueue retry", http.StatusBadGateway)
		return
	}
	if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
		log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
	}
	if err := h.bumpItemDetailVersion(r.Context(), item.ID); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", item.ID, err)
	}
	h.invalidateUserCaches(r.Context(), userID)
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, retryItemResponse{Status: "queued", ItemID: item.ID})
}

func (h *ItemHandler) RetryFromFactsBulk(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var body retryBulkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	itemIDs := normalizeBulkItemIDs(body.ItemIDs)
	if len(itemIDs) == 0 {
		http.Error(w, "item_ids is required", http.StatusBadRequest)
		return
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}

	result := runRetryBulk(
		r.Context(),
		itemIDs,
		func(ctx context.Context, itemID string) (retryBulkCandidate, error) {
			item, err := h.repo.ResetForFactsRetry(ctx, itemID, userID)
			if err != nil {
				return retryBulkCandidate{}, err
			}
			return retryBulkCandidate{
				ID:       item.ID,
				SourceID: item.SourceID,
				URL:      item.URL,
			}, nil
		},
		func(ctx context.Context, item retryBulkCandidate) error {
			return h.publisher.SendItemCreatedWithReasonE(ctx, item.ID, item.SourceID, item.URL, nil, "retry_from_facts")
		},
	)

	if result.QueuedCount > 0 {
		if err := h.bumpUserItemsVersion(r.Context(), userID); err != nil {
			log.Printf("items-list version bump failed user_id=%s err=%v", userID, err)
		}
		for _, itemID := range result.queuedItemIDs {
			if err := h.bumpItemDetailVersion(r.Context(), itemID); err != nil {
				log.Printf("item-detail version bump failed item_id=%s err=%v", itemID, err)
			}
		}
	}
	h.invalidateUserCaches(r.Context(), userID)
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, result)
}

func (h *ItemHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	q := r.URL.Query()
	var sourceID *string
	if v := q.Get("source_id"); v != "" {
		sourceID = &v
	}
	if h.publisher == nil {
		http.Error(w, "event publisher unavailable", http.StatusInternalServerError)
		return
	}

	items, err := h.repo.ListFailedForRetry(r.Context(), userID, sourceID)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	queued := 0
	failed := 0
	for _, item := range items {
		if err := h.publisher.SendItemCreatedWithReasonE(r.Context(), item.ID, item.SourceID, item.URL, nil, "retry_failed"); err != nil {
			failed++
			continue
		}
		queued++
	}

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, retryFailedResponse{
		Status:      "queued",
		SourceID:    sourceID,
		Matched:     len(items),
		QueuedCount: queued,
		FailedCount: failed,
	})
}
