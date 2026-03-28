package handler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type personaHistoryTestCache struct {
	values map[string][]byte
}

func newPersonaHistoryTestCache() *personaHistoryTestCache {
	return &personaHistoryTestCache{values: map[string][]byte{}}
}

func (c *personaHistoryTestCache) GetJSON(_ context.Context, key string, dst any) (bool, error) {
	raw, ok := c.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (c *personaHistoryTestCache) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.values[key] = raw
	return nil
}

func (c *personaHistoryTestCache) GetVersion(context.Context, string) (int64, error)  { return 0, nil }
func (c *personaHistoryTestCache) BumpVersion(context.Context, string) (int64, error) { return 0, nil }
func (c *personaHistoryTestCache) DeleteByPrefix(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (c *personaHistoryTestCache) Ping(context.Context) error { return nil }
func (c *personaHistoryTestCache) IncrMetric(context.Context, string, string, int64, time.Time, time.Duration) error {
	return nil
}
func (c *personaHistoryTestCache) SumMetrics(context.Context, string, time.Time, time.Time) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func TestRememberBriefingNavigatorPersona(t *testing.T) {
	cache := newPersonaHistoryTestCache()
	ctx := context.Background()
	rememberBriefingNavigatorPersona(ctx, cache, "u1", "editor")
	rememberBriefingNavigatorPersona(ctx, cache, "u1", "hype")
	rememberBriefingNavigatorPersona(ctx, cache, "u1", "editor")
	rememberBriefingNavigatorPersona(ctx, cache, "u1", "analyst")

	var got []string
	ok, err := cache.GetJSON(ctx, cacheKeyBriefingNavigatorPersonaHistory("u1"), &got)
	if err != nil {
		t.Fatalf("GetJSON returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected persona history to be stored")
	}
	want := []string{"analyst", "editor", "hype"}
	if len(got) != len(want) {
		t.Fatalf("history len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("history[%d] = %q, want %q (%#v)", i, got[i], want[i], got)
		}
	}
}

func TestSelectBriefingNavigatorPersonaUsesCacheHistory(t *testing.T) {
	cache := newPersonaHistoryTestCache()
	ctx := context.Background()
	if err := cache.SetJSON(ctx, cacheKeyBriefingNavigatorPersonaHistory("u1"), []string{"editor", "hype", "analyst"}, time.Hour); err != nil {
		t.Fatalf("SetJSON returned error: %v", err)
	}
	settings := &model.UserSettings{
		NavigatorPersonaMode: "random",
		NavigatorPersona:     "editor",
	}
	for i := 0; i < 20; i++ {
		got := selectBriefingNavigatorPersona(ctx, cache, "u1", settings)
		if got == "editor" || got == "hype" || got == "analyst" {
			t.Fatalf("selected blocked persona %q", got)
		}
	}
}
