package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/service"
)

const cacheKeyVersion = "v1"
const navigatorCacheKeyVersion = "v2"
const itemsListCacheSchemaVersion = 3
const itemDetailCacheSchemaVersion = 3

func cacheVersionKeyUserItems(userID string) string {
	return fmt.Sprintf("cache_version:user_items:%s", userID)
}

func cacheVersionKeyItemDetail(itemID string) string {
	return fmt.Sprintf("cache_version:item_detail:v3:%s", itemID)
}

func cacheVersionKeyUserSettings(userID string) string {
	return fmt.Sprintf("cache_version:user_settings:%s", userID)
}

func cacheVersionKeyUserPreferenceProfile(userID string) string {
	return fmt.Sprintf("cache_version:user_preference_profile:%s", userID)
}

func cacheVersionKeyUserLLMUsage(userID string) string {
	return service.UserLLMUsageCacheVersionKey(userID)
}

func cacheKeyItemsList(userID, status, sourceID, topic, genre, query, searchMode string, unreadOnly, readOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) string {
	return fmt.Sprintf(
		"%s:items:list:%s:sv=%d:status=%s:source=%s:topic=%s:genre=%s:q=%s:mode=%s:unread=%t:read=%t:fav=%t:later=%t:sort=%s:page=%d:size=%d",
		cacheKeyVersion,
		userID,
		itemsListCacheSchemaVersion,
		status,
		sourceID,
		topic,
		genre,
		query,
		searchMode,
		unreadOnly,
		readOnly,
		favoriteOnly,
		laterOnly,
		sort,
		page,
		pageSize,
	)
}

func cacheKeyItemsListVersioned(userID string, version int64, status, sourceID, topic, genre, query, searchMode string, unreadOnly, readOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) string {
	return fmt.Sprintf(
		"%s:items:list:%s:sv=%d:v=%d:status=%s:source=%s:topic=%s:genre=%s:q=%s:mode=%s:unread=%t:read=%t:fav=%t:later=%t:sort=%s:page=%d:size=%d",
		cacheKeyVersion,
		userID,
		itemsListCacheSchemaVersion,
		version,
		status,
		sourceID,
		topic,
		genre,
		query,
		searchMode,
		unreadOnly,
		readOnly,
		favoriteOnly,
		laterOnly,
		sort,
		page,
		pageSize,
	)
}

func cacheKeyReadingPlan(userID, window string, size int, diversifyTopics, excludeRead, excludeLater bool) string {
	return fmt.Sprintf("%s:items:reading-plan:%s:window=%s:size=%d:div=%t:exclude_read=%t:exclude_later=%t", cacheKeyVersion, userID, window, size, diversifyTopics, excludeRead, excludeLater)
}

func cacheKeyFocusQueue(userID, window string, size int, diversifyTopics, excludeLater bool) string {
	return fmt.Sprintf("%s:items:focus-queue:%s:window=%s:size=%d:div=%t:exclude_later=%t", cacheKeyVersion, userID, window, size, diversifyTopics, excludeLater)
}

func cacheKeyTriageQueue(userID, window string, size int, diversifyTopics, excludeLater bool) string {
	return fmt.Sprintf("%s:items:triage-queue:%s:window=%s:size=%d:div=%t:exclude_later=%t", cacheKeyVersion, userID, window, size, diversifyTopics, excludeLater)
}

func cacheKeyTriageAll(userID string) string {
	return fmt.Sprintf("%s:items:triage-all:%s", cacheKeyVersion, userID)
}

func cacheKeyRelated(userID, itemID string, limit int) string {
	return fmt.Sprintf("%s:items:related:%s:item=%s:limit=%d", cacheKeyVersion, userID, itemID, limit)
}

func cacheKeyBriefingToday(userID string, size int) string {
	return fmt.Sprintf("%s:briefing:today:%s:size=%d", cacheKeyVersion, userID, size)
}

func cacheKeyBriefingNavigator(userID, persona, model string, preview bool) string {
	return fmt.Sprintf("%s:briefing:navigator:%s:persona=%s:model=%s:preview=%t", navigatorCacheKeyVersion, userID, persona, model, preview)
}

func cacheKeyItemNavigator(userID, itemID, persona, model string, preview bool) string {
	return fmt.Sprintf("%s:item:navigator:%s:item=%s:persona=%s:model=%s:preview=%t", navigatorCacheKeyVersion, userID, itemID, persona, model, preview)
}

func cacheKeySourceNavigator(userID, persona, model string) string {
	return fmt.Sprintf("%s:source:navigator:%s:persona=%s:model=%s", navigatorCacheKeyVersion, userID, persona, model)
}

func cacheKeyItemDetailVersioned(userID, itemID string, version int64) string {
	return fmt.Sprintf("%s:items:detail:%s:sv=%d:item=%s:v=%d", cacheKeyVersion, userID, itemDetailCacheSchemaVersion, itemID, version)
}

func cacheKeySettingsGetVersioned(userID string, version int64) string {
	return fmt.Sprintf("%s:settings:get:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyPreferenceProfile(userID string, version int64) string {
	return fmt.Sprintf("%s:settings:preference_profile:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyPreferenceProfileSummary(userID string, version int64) string {
	return fmt.Sprintf("%s:settings:preference_profile_summary:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyDashboard(userID string, llmDays, topicLimit, digestLimit int) string {
	return fmt.Sprintf("%s:dashboard:snapshot:%s:llm=%d:topic=%d:digest=%d", cacheKeyVersion, userID, llmDays, topicLimit, digestLimit)
}

func cacheKeyDashboardPart(userID, part string, p1, p2 int) string {
	switch part {
	case "sources", "itemstats", "failedpreview":
		return fmt.Sprintf("%s:dashboard:part:%s:%s", cacheKeyVersion, userID, part)
	case "digests":
		return fmt.Sprintf("%s:dashboard:part:%s:%s:limit=%d", cacheKeyVersion, userID, part, p1)
	case "llm":
		return fmt.Sprintf("%s:dashboard:part:%s:%s:days=%d", cacheKeyVersion, userID, part, p1)
	case "topics":
		return fmt.Sprintf("%s:dashboard:part:%s:%s:limit=%d", cacheKeyVersion, userID, part, p1)
	default:
		return fmt.Sprintf("%s:dashboard:part:%s:%s:%d:%d", cacheKeyVersion, userID, part, p1, p2)
	}
}

func cacheKeyAsk(userID, query, answerModel, embeddingModel string, days int, unreadOnly bool, limit int, sourceIDs []string) string {
	normalizedSourceIDs := make([]string, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		v := strings.TrimSpace(sourceID)
		if v != "" {
			normalizedSourceIDs = append(normalizedSourceIDs, v)
		}
	}
	sort.Strings(normalizedSourceIDs)
	sum := sha256.Sum256([]byte(strings.TrimSpace(query)))
	return fmt.Sprintf(
		"%s:ask:%s:q=%s:model=%s:emb=%s:days=%d:unread=%t:limit=%d:sources=%s",
		cacheKeyVersion,
		userID,
		hex.EncodeToString(sum[:8]),
		answerModel,
		embeddingModel,
		days,
		unreadOnly,
		limit,
		strings.Join(normalizedSourceIDs, ","),
	)
}

func cacheKeyAskNavigator(userID, query, answer, persona, model string) string {
	queryHash := sha256.Sum256([]byte(strings.TrimSpace(query)))
	answerHash := sha256.Sum256([]byte(strings.TrimSpace(answer)))
	return fmt.Sprintf(
		"%s:ask:navigator:%s:q=%s:a=%s:persona=%s:model=%s",
		navigatorCacheKeyVersion,
		userID,
		hex.EncodeToString(queryHash[:8]),
		hex.EncodeToString(answerHash[:8]),
		persona,
		model,
	)
}

func cacheKeyLLMUsageDailySummaryVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:daily:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageDailySummaryMonthVersioned(userID string, version int64, month string) string {
	return fmt.Sprintf("%s:llm_usage:daily:%s:v=%d:month=%s", cacheKeyVersion, userID, version, month)
}

func cacheKeyLLMUsageModelSummaryVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:model:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageModelSummaryMonthVersioned(userID string, version int64, month string) string {
	return fmt.Sprintf("%s:llm_usage:model:%s:v=%d:month=%s", cacheKeyVersion, userID, version, month)
}

func cacheKeyLLMUsageAnalysisVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:analysis:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageProviderCurrentMonthVersioned(userID string, version int64, month string) string {
	return fmt.Sprintf("%s:llm_usage:provider_current_month:%s:v=%d:month=%s", cacheKeyVersion, userID, version, month)
}

func cacheKeyLLMUsagePurposeCurrentMonthVersioned(userID string, version int64, month string) string {
	return fmt.Sprintf("%s:llm_usage:purpose_current_month:%s:v=%d:month=%s", cacheKeyVersion, userID, version, month)
}

func cacheKeyLLMUsageExecutionCurrentMonthVersioned(userID string, version int64, month string) string {
	return fmt.Sprintf("%s:llm_usage:execution_current_month:%s:v=%d:month=%s", cacheKeyVersion, userID, version, month)
}

func cacheKeyLLMUsageExecutionSummaryVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:execution:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageValueMetricsCurrentMonthVersioned(userID string, version int64, month string) string {
	return fmt.Sprintf("%s:llm_usage:value_metrics_current_month:%s:v=%d:month=%s", cacheKeyVersion, userID, version, month)
}

func cacheKeyLLMUsageListVersioned(userID string, version int64, limit int) string {
	return fmt.Sprintf("%s:llm_usage:list:%s:v=%d:limit=%d", cacheKeyVersion, userID, version, limit)
}

func cacheKeyLLMUsageListMonthVersioned(userID string, version int64, limit int, month string) string {
	return fmt.Sprintf("%s:llm_usage:list:%s:v=%d:limit=%d:month=%s", cacheKeyVersion, userID, version, limit, month)
}

func cacheUserInvalidatePrefixes(userID string) []string {
	return []string{
		fmt.Sprintf("%s:items:list:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:reading-plan:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:focus-queue:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:triage-queue:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:triage-all:%s", cacheKeyVersion, userID),
		fmt.Sprintf("%s:briefing:today:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:dashboard:snapshot:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:dashboard:part:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:ask:%s:", cacheKeyVersion, userID),
	}
}
