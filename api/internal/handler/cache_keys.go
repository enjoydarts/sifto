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

func cacheVersionKeyUserItems(userID string) string {
	return fmt.Sprintf("cache_version:user_items:%s", userID)
}

func cacheVersionKeyItemDetail(itemID string) string {
	return fmt.Sprintf("cache_version:item_detail:v2:%s", itemID)
}

func cacheVersionKeyUserSettings(userID string) string {
	return fmt.Sprintf("cache_version:user_settings:%s", userID)
}

func cacheVersionKeyUserLLMUsage(userID string) string {
	return service.UserLLMUsageCacheVersionKey(userID)
}

func cacheKeyItemsList(userID, status, sourceID, topic, query string, unreadOnly, readOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) string {
	return fmt.Sprintf(
		"%s:items:list:%s:status=%s:source=%s:topic=%s:q=%s:unread=%t:read=%t:fav=%t:later=%t:sort=%s:page=%d:size=%d",
		cacheKeyVersion,
		userID,
		status,
		sourceID,
		topic,
		query,
		unreadOnly,
		readOnly,
		favoriteOnly,
		laterOnly,
		sort,
		page,
		pageSize,
	)
}

func cacheKeyItemsListVersioned(userID string, version int64, status, sourceID, topic, query string, unreadOnly, readOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) string {
	return fmt.Sprintf(
		"%s:items:list:%s:v=%d:status=%s:source=%s:topic=%s:q=%s:unread=%t:read=%t:fav=%t:later=%t:sort=%s:page=%d:size=%d",
		cacheKeyVersion,
		userID,
		version,
		status,
		sourceID,
		topic,
		query,
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

func cacheKeyItemDetailVersioned(userID, itemID string, version int64) string {
	return fmt.Sprintf("%s:items:detail:%s:item=%s:v=%d", cacheKeyVersion, userID, itemID, version)
}

func cacheKeySettingsGetVersioned(userID string, version int64) string {
	return fmt.Sprintf("%s:settings:get:%s:v=%d", cacheKeyVersion, userID, version)
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

func cacheKeyLLMUsageDailySummaryVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:daily:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageModelSummaryVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:model:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageAnalysisVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:analysis:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageProviderCurrentMonthVersioned(userID string, version int64) string {
	return fmt.Sprintf("%s:llm_usage:provider_current_month:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyLLMUsagePurposeCurrentMonthVersioned(userID string, version int64) string {
	return fmt.Sprintf("%s:llm_usage:purpose_current_month:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyLLMUsageExecutionCurrentMonthVersioned(userID string, version int64) string {
	return fmt.Sprintf("%s:llm_usage:execution_current_month:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyLLMUsageExecutionSummaryVersioned(userID string, version int64, days int) string {
	return fmt.Sprintf("%s:llm_usage:execution:%s:v=%d:days=%d", cacheKeyVersion, userID, version, days)
}

func cacheKeyLLMUsageValueMetricsCurrentMonthVersioned(userID string, version int64) string {
	return fmt.Sprintf("%s:llm_usage:value_metrics_current_month:%s:v=%d", cacheKeyVersion, userID, version)
}

func cacheKeyLLMUsageListVersioned(userID string, version int64, limit int) string {
	return fmt.Sprintf("%s:llm_usage:list:%s:v=%d:limit=%d", cacheKeyVersion, userID, version, limit)
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
