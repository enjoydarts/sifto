package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type JSONCache interface {
	GetJSON(ctx context.Context, key string, dst any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
}

type NoopJSONCache struct{}

func (NoopJSONCache) GetJSON(context.Context, string, any) (bool, error) { return false, nil }
func (NoopJSONCache) SetJSON(context.Context, string, any, time.Duration) error {
	return nil
}

type RedisJSONCache struct {
	client *redis.Client
	prefix string
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
