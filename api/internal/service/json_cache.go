package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type JSONCache interface {
	GetJSON(ctx context.Context, key string, dst any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	GetVersion(ctx context.Context, key string) (int64, error)
	BumpVersion(ctx context.Context, key string) (int64, error)
	DeleteByPrefix(ctx context.Context, prefix string, limit int64) (int64, error)
	Ping(ctx context.Context) error
	IncrMetric(ctx context.Context, namespace, field string, delta int64, now time.Time, ttl time.Duration) error
	SumMetrics(ctx context.Context, namespace string, from, to time.Time) (map[string]int64, error)
}

type NoopJSONCache struct{}

func (NoopJSONCache) GetJSON(context.Context, string, any) (bool, error) { return false, nil }
func (NoopJSONCache) SetJSON(context.Context, string, any, time.Duration) error {
	return nil
}
func (NoopJSONCache) GetVersion(context.Context, string) (int64, error)            { return 0, nil }
func (NoopJSONCache) BumpVersion(context.Context, string) (int64, error)           { return 0, nil }
func (NoopJSONCache) DeleteByPrefix(context.Context, string, int64) (int64, error) { return 0, nil }
func (NoopJSONCache) Ping(context.Context) error                                   { return nil }
func (NoopJSONCache) IncrMetric(context.Context, string, string, int64, time.Time, time.Duration) error {
	return nil
}
func (NoopJSONCache) SumMetrics(context.Context, string, time.Time, time.Time) (map[string]int64, error) {
	return map[string]int64{}, nil
}

type RedisJSONCache struct {
	client *redis.Client
	prefix string
}

func UserLLMUsageCacheVersionKey(userID string) string {
	return fmt.Sprintf("cache_version:user_llm_usage:%s", strings.TrimSpace(userID))
}

func BumpUserLLMUsageCacheVersion(ctx context.Context, cache JSONCache, userID string) error {
	if cache == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	_, err := cache.BumpVersion(ctx, UserLLMUsageCacheVersionKey(userID))
	return err
}

func NewJSONCacheFromEnv() (JSONCache, error) {
	url := strings.TrimSpace(os.Getenv("UPSTASH_REDIS_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("REDIS_URL"))
	}
	if url == "" {
		return NoopJSONCache{}, nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	prefix := strings.TrimSpace(os.Getenv("REDIS_CACHE_PREFIX"))
	if prefix == "" {
		prefix = "sifto"
	}
	return &RedisJSONCache{client: client, prefix: prefix}, nil
}

func RedisClientFromCache(cache JSONCache) (*redis.Client, string) {
	redisCache, ok := cache.(*RedisJSONCache)
	if !ok || redisCache == nil {
		return nil, ""
	}
	return redisCache.client, redisCache.prefix
}

func (c *RedisJSONCache) key(k string) string {
	if c == nil {
		return k
	}
	if c.prefix == "" {
		return k
	}
	return c.prefix + ":" + k
}

func (c *RedisJSONCache) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	if c == nil || c.client == nil {
		return false, nil
	}
	s, err := c.client.Get(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(s), dst); err != nil {
		return false, err
	}
	return true, nil
}

func (c *RedisJSONCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.key(key), b, ttl).Err()
}

func (c *RedisJSONCache) GetVersion(ctx context.Context, key string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, nil
	}
	value, err := c.client.Get(ctx, c.key(key)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return value, nil
}

func (c *RedisJSONCache) BumpVersion(ctx context.Context, key string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, nil
	}
	return c.client.Incr(ctx, c.key(key)).Result()
}

func (c *RedisJSONCache) DeleteByPrefix(ctx context.Context, prefix string, limit int64) (int64, error) {
	if c == nil || c.client == nil || strings.TrimSpace(prefix) == "" {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	pattern := c.key(prefix) + "*"
	var (
		cursor  uint64
		deleted int64
	)
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return deleted, err
		}
		if len(keys) > 0 {
			pipe := c.client.Pipeline()
			for _, k := range keys {
				if deleted >= limit {
					break
				}
				pipe.Del(ctx, k)
				deleted++
			}
			if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
				return deleted, err
			}
		}
		if nextCursor == 0 || deleted >= limit {
			break
		}
		cursor = nextCursor
	}
	return deleted, nil
}

func (c *RedisJSONCache) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Ping(ctx).Err()
}

func metricBucketKey(namespace string, t time.Time) string {
	return fmt.Sprintf("metrics:%s:%s", namespace, t.UTC().Truncate(time.Minute).Format("200601021504"))
}

func (c *RedisJSONCache) IncrMetric(ctx context.Context, namespace, field string, delta int64, now time.Time, ttl time.Duration) error {
	if c == nil || c.client == nil || namespace == "" || field == "" || delta == 0 {
		return nil
	}
	key := c.key(metricBucketKey(namespace, now))
	_, err := c.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HIncrBy(ctx, key, field, delta)
		if ttl > 0 {
			p.Expire(ctx, key, ttl)
		}
		return nil
	})
	return err
}

func (c *RedisJSONCache) SumMetrics(ctx context.Context, namespace string, from, to time.Time) (map[string]int64, error) {
	out := map[string]int64{}
	if c == nil || c.client == nil || namespace == "" {
		return out, nil
	}
	start := from.UTC().Truncate(time.Minute)
	end := to.UTC().Truncate(time.Minute)
	if end.Before(start) {
		return out, nil
	}
	keys := make([]string, 0, int(end.Sub(start)/time.Minute)+1)
	for t := start; !t.After(end); t = t.Add(time.Minute) {
		keys = append(keys, c.key(metricBucketKey(namespace, t)))
	}
	pipe := c.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, 0, len(keys))
	for _, k := range keys {
		cmds = append(cmds, pipe.HGetAll(ctx, k))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}
	for _, cmd := range cmds {
		m, e := cmd.Result()
		if e != nil && e != redis.Nil {
			continue
		}
		for k, v := range m {
			n, convErr := strconv.ParseInt(v, 10, 64)
			if convErr != nil {
				continue
			}
			out[k] += n
		}
	}
	return out, nil
}
