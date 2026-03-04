package handler

import "fmt"

const cacheKeyVersion = "v1"

func cacheKeyItemsList(userID, status, sourceID, topic string, unreadOnly, favoriteOnly, laterOnly bool, sort string, page, pageSize int) string {
	return fmt.Sprintf(
		"%s:items:list:%s:status=%s:source=%s:topic=%s:unread=%t:fav=%t:later=%t:sort=%s:page=%d:size=%d",
		cacheKeyVersion,
		userID,
		status,
		sourceID,
		topic,
		unreadOnly,
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

func cacheKeyTriageAll(userID string) string {
	return fmt.Sprintf("%s:items:triage-all:%s", cacheKeyVersion, userID)
}

func cacheKeyRelated(userID, itemID string, limit int) string {
	return fmt.Sprintf("%s:items:related:%s:item=%s:limit=%d", cacheKeyVersion, userID, itemID, limit)
}

func cacheKeyBriefingToday(userID string, size int) string {
	return fmt.Sprintf("%s:briefing:today:%s:size=%d", cacheKeyVersion, userID, size)
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

func cacheUserInvalidatePrefixes(userID string) []string {
	return []string{
		fmt.Sprintf("%s:items:list:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:reading-plan:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:focus-queue:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:items:triage-all:%s", cacheKeyVersion, userID),
		fmt.Sprintf("%s:briefing:today:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:dashboard:snapshot:%s:", cacheKeyVersion, userID),
		fmt.Sprintf("%s:dashboard:part:%s:", cacheKeyVersion, userID),
	}
}
