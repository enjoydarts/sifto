package handler

import (
	"context"
	"testing"
	"time"
)

type testJSONCache struct {
	versions map[string]int64
	getCalls []string
	setCalls []string
}

func newTestJSONCache() *testJSONCache {
	return &testJSONCache{versions: map[string]int64{}}
}

func (c *testJSONCache) GetJSON(context.Context, string, any) (bool, error) { return false, nil }
func (c *testJSONCache) SetJSON(_ context.Context, key string, _ any, _ time.Duration) error {
	c.setCalls = append(c.setCalls, key)
	return nil
}
func (c *testJSONCache) GetVersion(_ context.Context, key string) (int64, error) {
	c.getCalls = append(c.getCalls, key)
	return c.versions[key], nil
}
func (c *testJSONCache) BumpVersion(_ context.Context, key string) (int64, error) {
	c.versions[key]++
	return c.versions[key], nil
}
func (c *testJSONCache) DeleteByPrefix(context.Context, string, int64) (int64, error) { return 0, nil }
func (c *testJSONCache) Ping(context.Context) error                                   { return nil }
func (c *testJSONCache) IncrMetric(context.Context, string, string, int64, time.Time, time.Duration) error {
	return nil
}
func (c *testJSONCache) SumMetrics(context.Context, string, time.Time, time.Time) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func TestItemListCacheKeyUsesVersion(t *testing.T) {
	cache := newTestJSONCache()
	cache.versions[cacheVersionKeyUserItems("u1")] = 7
	handler := &ItemHandler{cache: cache}

	key, err := handler.itemsListCacheKey(context.Background(), "u1", "summarized", "src-1", "go", "analysis", "openai", "and", true, false, true, false, "score", 2, 50)
	if err != nil {
		t.Fatalf("itemsListCacheKey returned error: %v", err)
	}
	want := "v1:items:list:u1:sv=3:v=7:status=summarized:source=src-1:topic=go:genre=analysis:q=openai:mode=and:unread=true:read=false:fav=true:later=false:sort=score:page=2:size=50"
	if key != want {
		t.Fatalf("itemsListCacheKey = %q, want %q", key, want)
	}
}

func TestItemListCacheTTLBySort(t *testing.T) {
	tests := []struct {
		sort string
		want time.Duration
	}{
		{sort: "newest", want: time.Minute},
		{sort: "score", want: 2 * time.Minute},
		{sort: "personal_score", want: 5 * time.Minute},
		{sort: "unexpected", want: time.Minute},
	}

	for _, tt := range tests {
		if got := itemsListCacheTTLForSort(tt.sort); got != tt.want {
			t.Fatalf("itemsListCacheTTLForSort(%q) = %s, want %s", tt.sort, got, tt.want)
		}
	}
}

func TestBumpUserItemsVersion(t *testing.T) {
	cache := newTestJSONCache()
	handler := &ItemHandler{cache: cache}

	if err := handler.bumpUserItemsVersion(context.Background(), "u1"); err != nil {
		t.Fatalf("first bumpUserItemsVersion returned error: %v", err)
	}
	if err := handler.bumpUserItemsVersion(context.Background(), "u1"); err != nil {
		t.Fatalf("second bumpUserItemsVersion returned error: %v", err)
	}

	if got, want := cache.versions[cacheVersionKeyUserItems("u1")], int64(2); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
}

func TestItemDetailCacheKeyUsesVersion(t *testing.T) {
	cache := newTestJSONCache()
	cache.versions[cacheVersionKeyItemDetail("item-1")] = 3
	handler := &ItemHandler{cache: cache}

	key, err := handler.itemDetailCacheKey(context.Background(), "u1", "item-1")
	if err != nil {
		t.Fatalf("itemDetailCacheKey returned error: %v", err)
	}
	want := "v1:items:detail:u1:sv=3:item=item-1:v=3"
	if key != want {
		t.Fatalf("itemDetailCacheKey = %q, want %q", key, want)
	}
}

func TestBumpItemDetailVersion(t *testing.T) {
	cache := newTestJSONCache()
	handler := &ItemHandler{cache: cache}

	if err := handler.bumpItemDetailVersion(context.Background(), "item-1"); err != nil {
		t.Fatalf("first bumpItemDetailVersion returned error: %v", err)
	}
	if err := handler.bumpItemDetailVersion(context.Background(), "item-1"); err != nil {
		t.Fatalf("second bumpItemDetailVersion returned error: %v", err)
	}

	if got, want := cache.versions[cacheVersionKeyItemDetail("item-1")], int64(2); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
}
