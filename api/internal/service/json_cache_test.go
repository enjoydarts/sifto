package service

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestJSONCacheVersionNoop(t *testing.T) {
	cache := NoopJSONCache{}

	version, err := cache.GetVersion(context.Background(), "cache_version:user_items:u1")
	if err != nil {
		t.Fatalf("GetVersion returned error: %v", err)
	}
	if version != 0 {
		t.Fatalf("GetVersion = %d, want 0", version)
	}

	version, err = cache.BumpVersion(context.Background(), "cache_version:user_items:u1")
	if err != nil {
		t.Fatalf("BumpVersion returned error: %v", err)
	}
	if version != 0 {
		t.Fatalf("BumpVersion = %d, want 0", version)
	}
}

func TestJSONCacheVersionRedis(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "redis:6379", DB: 15})
	t.Cleanup(func() {
		_ = client.FlushDB(ctx).Err()
		_ = client.Close()
	})
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("FlushDB failed: %v", err)
	}

	cache := &RedisJSONCache{client: client, prefix: "sifto-test"}
	versionKey := "cache_version:user_items:u1"

	version, err := cache.GetVersion(ctx, versionKey)
	if err != nil {
		t.Fatalf("GetVersion returned error: %v", err)
	}
	if version != 0 {
		t.Fatalf("initial GetVersion = %d, want 0", version)
	}

	version, err = cache.BumpVersion(ctx, versionKey)
	if err != nil {
		t.Fatalf("first BumpVersion returned error: %v", err)
	}
	if version != 1 {
		t.Fatalf("first BumpVersion = %d, want 1", version)
	}

	version, err = cache.BumpVersion(ctx, versionKey)
	if err != nil {
		t.Fatalf("second BumpVersion returned error: %v", err)
	}
	if version != 2 {
		t.Fatalf("second BumpVersion = %d, want 2", version)
	}

	raw, err := client.Get(ctx, "sifto-test:"+versionKey).Result()
	if err != nil {
		t.Fatalf("redis Get failed: %v", err)
	}
	if raw != "2" {
		t.Fatalf("stored version = %q, want %q", raw, "2")
	}
}

func TestBumpUserLLMUsageCacheVersion(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "redis:6379", DB: 15})
	t.Cleanup(func() {
		_ = client.FlushDB(ctx).Err()
		_ = client.Close()
	})
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("FlushDB failed: %v", err)
	}

	cache := &RedisJSONCache{client: client, prefix: "sifto-test"}
	if err := BumpUserLLMUsageCacheVersion(ctx, cache, "u1"); err != nil {
		t.Fatalf("first BumpUserLLMUsageCacheVersion returned error: %v", err)
	}
	if err := BumpUserLLMUsageCacheVersion(ctx, cache, "u1"); err != nil {
		t.Fatalf("second BumpUserLLMUsageCacheVersion returned error: %v", err)
	}

	raw, err := client.Get(ctx, "sifto-test:"+UserLLMUsageCacheVersionKey("u1")).Result()
	if err != nil {
		t.Fatalf("redis Get failed: %v", err)
	}
	if raw != "2" {
		t.Fatalf("stored version = %q, want %q", raw, "2")
	}
}
