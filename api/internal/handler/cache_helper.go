package handler

import (
	"context"
	"log"
	"time"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func cachedFetch[T any](ctx context.Context, cache service.JSONCache, key string, ttl time.Duration, fetchFn func() (T, error)) (T, error) {
	var zero T
	if cache != nil {
		var cached T
		if ok, err := cache.GetJSON(ctx, key, &cached); err == nil && ok {
			return cached, nil
		}
	}
	result, err := fetchFn()
	if err != nil {
		return zero, err
	}
	if cache != nil {
		_ = cache.SetJSON(ctx, key, result, ttl)
	}
	return result, nil
}

type cacheFetchOptions struct {
	cacheBust           bool
	cacheKeyErr         error
	metricPrefix        string
	userID              string
	counter             *cacheCounter
	logKeyPrefix        string
	skipCacheSet        bool
	cacheSetTTLOverride time.Duration
}

func cachedFetchWithOpts[T any](ctx context.Context, cache service.JSONCache, key string, ttl time.Duration, fetchFn func() (T, error), opts cacheFetchOptions) (T, error) {
	var zero T
	if opts.counter != nil && opts.cacheKeyErr != nil {
		opts.counter.errors.Add(1)
		incrCacheMetric(ctx, cache, opts.userID, opts.metricPrefix+".error")
		log.Printf("%s cache key failed user_id=%s err=%v", opts.logKeyPrefix, opts.userID, opts.cacheKeyErr)
	}
	if cache != nil && !opts.cacheBust && opts.cacheKeyErr == nil {
		var cached T
		if ok, err := cache.GetJSON(ctx, key, &cached); err == nil && ok {
			if opts.counter != nil {
				opts.counter.hits.Add(1)
			}
			if opts.metricPrefix != "" {
				incrCacheMetric(ctx, cache, opts.userID, opts.metricPrefix+".hit")
			}
			return cached, nil
		} else if err != nil {
			if opts.counter != nil {
				opts.counter.errors.Add(1)
			}
			if opts.metricPrefix != "" {
				incrCacheMetric(ctx, cache, opts.userID, opts.metricPrefix+".error")
			}
			log.Printf("%s cache get failed user_id=%s key=%s err=%v", opts.logKeyPrefix, opts.userID, key, err)
		}
		if opts.counter != nil {
			opts.counter.misses.Add(1)
		}
		if opts.metricPrefix != "" {
			incrCacheMetric(ctx, cache, opts.userID, opts.metricPrefix+".miss")
		}
	} else if opts.cacheBust {
		if opts.counter != nil {
			opts.counter.bypass.Add(1)
		}
		if opts.metricPrefix != "" && cache != nil {
			incrCacheMetric(ctx, cache, opts.userID, opts.metricPrefix+".bypass")
		}
	}

	result, err := fetchFn()
	if err != nil {
		return zero, err
	}

	if !opts.skipCacheSet && cache != nil && opts.cacheKeyErr == nil {
		setTTL := ttl
		if opts.cacheSetTTLOverride > 0 {
			setTTL = opts.cacheSetTTLOverride
		}
		if err := cache.SetJSON(ctx, key, result, setTTL); err != nil {
			if opts.counter != nil {
				opts.counter.errors.Add(1)
			}
			if opts.metricPrefix != "" {
				incrCacheMetric(ctx, cache, opts.userID, opts.metricPrefix+".error")
			}
			log.Printf("%s cache set failed user_id=%s key=%s err=%v", opts.logKeyPrefix, opts.userID, key, err)
		}
	}
	return result, nil
}
