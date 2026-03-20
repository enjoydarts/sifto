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
	publisher       *service.EventPublisher
	cache           service.JSONCache
	detail          *service.ItemDetailService
}

const itemsListCacheTTL = 30 * time.Second
const focusQueueCacheTTL = 60 * time.Second
const triageAllCacheTTL = 90 * time.Second
const relatedItemsCacheTTL = 5 * time.Minute
const itemDetailCacheTTL = 5 * time.Minute

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
	publisher *service.EventPublisher,
	cache service.JSONCache,
) *ItemHandler {
	return &ItemHandler{
		repo:            repo,
		sourceRepo:      sourceRepo,
		readingGoalRepo: readingGoalRepo,
		streakRepo:      streakRepo,
		snapshotRepo:    snapshotRepo,
		prefProfileRepo: prefProfileRepo,
		reviewQueueRepo: reviewQueueRepo,
		publisher:       publisher,
		cache:           cache,
		detail:          service.NewItemDetailService(repo),
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

func (h *ItemHandler) itemsListCacheKey(ctx context.Context, userID, status, sourceID, topic, searchQuery string, unreadOnly, readOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) (string, error) {
	version := int64(0)
	if h.cache != nil {
		var err error
		version, err = h.cache.GetVersion(ctx, cacheVersionKeyUserItems(userID))
		if err != nil {
			return "", err
		}
	}
	return cacheKeyItemsListVersioned(userID, version, status, sourceID, topic, searchQuery, unreadOnly, readOnly, favoriteOnly, laterOnly, sort, page, pageSize), nil
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
	var status, sourceID, topic *string
	if v := q.Get("status"); v != "" {
		status = &v
	}
	if v := q.Get("source_id"); v != "" {
		sourceID = &v
	}
	if v := q.Get("topic"); v != "" {
		topic = &v
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
	cacheKey, cacheKeyErr := h.itemsListCacheKey(r.Context(), userID, q.Get("status"), q.Get("source_id"), q.Get("topic"), searchQuery, unreadOnly, readOnly, favoriteOnly, laterOnly, sort, page, pageSize)
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
	resp, err := h.repo.ListPage(r.Context(), userID, repository.ItemListParams{
		Status:       status,
		SourceID:     sourceID,
		Topic:        topic,
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
	if sort == "personal_score" && resp != nil && len(resp.Items) > 0 {
		h.applyPersonalScoreSort(r.Context(), userID, resp)
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
	writeJSON(w, map[string]any{
		"items": rows,
		"limit": limit,
	})
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
	writeJSON(w, map[string]any{
		"days":  days,
		"limit": limit,
		"items": rows,
	})
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
	if h.cache != nil && !cacheBust {
		var cached model.ReadingPlanResponse
		if ok, err := h.cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			readingPlanCacheCounter.hits.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.hit")
			log.Printf("reading-plan cache hit user_id=%s key=%s", userID, cacheKey)
			writeJSON(w, &cached)
			return
		} else if err != nil {
			readingPlanCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.error")
			log.Printf("reading-plan cache get failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
		readingPlanCacheCounter.misses.Add(1)
		incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.miss")
		log.Printf("reading-plan cache miss user_id=%s key=%s", userID, cacheKey)
	} else if cacheBust {
		readingPlanCacheCounter.bypass.Add(1)
		if h.cache != nil {
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.bypass")
		}
		log.Printf("reading-plan cache bypass user_id=%s key=%s", userID, cacheKey)
	}

	resp, err := h.repo.ReadingPlan(r.Context(), userID, params)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if h.cache != nil && resp != nil {
		if err := h.cache.SetJSON(r.Context(), cacheKey, resp, 120*time.Second); err != nil {
			readingPlanCacheCounter.errors.Add(1)
			incrCacheMetric(r.Context(), h.cache, userID, "reading_plan.error")
			log.Printf("reading-plan cache set failed user_id=%s key=%s err=%v", userID, cacheKey, err)
		}
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
		out := map[string]any{
			"items":       []model.Item{},
			"size":        size,
			"window":      window,
			"completed":   0,
			"remaining":   0,
			"total":       0,
			"source_pool": 0,
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
	out := map[string]any{
		"items":            items,
		"size":             size,
		"window":           resp.Window,
		"completed":        completed,
		"remaining":        len(items) - completed,
		"total":            len(items),
		"source_pool":      resp.SourcePoolCount,
		"diversify_topics": resp.DiversifyTopics,
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
	out := map[string]any{
		"items":            items,
		"size":             len(items),
		"window":           "all",
		"completed":        0,
		"remaining":        len(items),
		"total":            len(items),
		"source_pool":      0,
		"diversify_topics": false,
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
	writeJSON(w, item)
}

func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id, userID); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.bumpItemDetailVersion(r.Context(), id); err != nil {
		log.Printf("item-detail version bump failed item_id=%s err=%v", id, err)
	}
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
	h.invalidateUserCaches(r.Context(), userID)
	item, err := h.getItemDetail(r.Context(), userID, id, true)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, item)
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
	out := map[string]any{
		"items":    items,
		"clusters": clusters,
		"limit":    limit,
		"item_id":  id,
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
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_read": true})
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
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_read": false})
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
		writeJSON(w, map[string]any{"status": "ok", "updated_count": updated})
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
	writeJSON(w, map[string]any{"status": "ok", "updated_count": updated})
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
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_later": true})
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
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, map[string]any{"item_id": id, "is_later": false})
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
	writeJSON(w, map[string]any{"status": "ok", "updated_count": updated})
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
	if h.reviewQueueRepo != nil && body.IsFavorite {
		_ = h.reviewQueueRepo.EnqueueDefault(r.Context(), userID, id, "favorite", time.Now())
	}
	h.invalidateUserCaches(r.Context(), userID)
	writeJSON(w, fb)
}

func (h *ItemHandler) Retry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetForRetry(r.Context(), id, userID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	summaryEmpty := item.Summary == nil || strings.TrimSpace(*item.Summary) == ""
	retryable := item.Status == "failed" || item.Status == "fetched" || item.Status == "facts_extracted" || item.Status == "summarized"
	if !retryable && !(item.Status == "summarized" && summaryEmpty) {
		http.Error(w, "item is not retryable", http.StatusConflict)
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
	writeJSON(w, map[string]any{
		"status":  "queued",
		"item_id": item.ID,
	})
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
	writeJSON(w, map[string]any{
		"status":  "queued",
		"item_id": item.ID,
	})
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
	writeJSON(w, map[string]any{
		"status":       "queued",
		"source_id":    sourceID,
		"matched":      len(items),
		"queued_count": queued,
		"failed_count": failed,
	})
}
