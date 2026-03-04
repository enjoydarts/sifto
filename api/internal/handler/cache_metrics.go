package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/service"
)

const userCacheMetricTTL = 24 * time.Hour

func incrCacheMetric(ctx context.Context, cache service.JSONCache, userID, field string) {
	if cache == nil || field == "" {
		return
	}
	now := time.Now()
	_ = cache.IncrMetric(ctx, "cache", field, 1, now, cacheMetricTTL)
	if userID != "" {
		_ = cache.IncrMetric(ctx, fmt.Sprintf("cache_user:%s", userID), field, 1, now, userCacheMetricTTL)
	}
}
