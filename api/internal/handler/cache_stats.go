package handler

import "sync/atomic"

type cacheCounter struct {
	hits   atomic.Int64
	misses atomic.Int64
	bypass atomic.Int64
	errors atomic.Int64
}

type cacheStatsSnapshot struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
	Bypass int64 `json:"bypass"`
	Errors int64 `json:"errors"`
}

var (
	dashboardCacheCounter   cacheCounter
	readingPlanCacheCounter cacheCounter
	itemsListCacheCounter   cacheCounter
)

func cacheStatsSnapshotAll() map[string]cacheStatsSnapshot {
	return map[string]cacheStatsSnapshot{
		"dashboard": {
			Hits:   dashboardCacheCounter.hits.Load(),
			Misses: dashboardCacheCounter.misses.Load(),
			Bypass: dashboardCacheCounter.bypass.Load(),
			Errors: dashboardCacheCounter.errors.Load(),
		},
		"reading_plan": {
			Hits:   readingPlanCacheCounter.hits.Load(),
			Misses: readingPlanCacheCounter.misses.Load(),
			Bypass: readingPlanCacheCounter.bypass.Load(),
			Errors: readingPlanCacheCounter.errors.Load(),
		},
		"items_list": {
			Hits:   itemsListCacheCounter.hits.Load(),
			Misses: itemsListCacheCounter.misses.Load(),
			Bypass: itemsListCacheCounter.bypass.Load(),
			Errors: itemsListCacheCounter.errors.Load(),
		},
	}
}
