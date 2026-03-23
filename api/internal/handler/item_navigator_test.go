package handler

import "testing"

func TestCacheKeyItemNavigatorVariesByItemPersonaModelAndPreview(t *testing.T) {
	k1 := cacheKeyItemNavigator("u1", "item-1", "editor", "gpt-5", false)
	k2 := cacheKeyItemNavigator("u1", "item-2", "editor", "gpt-5", false)
	k3 := cacheKeyItemNavigator("u1", "item-1", "snark", "gpt-5", false)
	k4 := cacheKeyItemNavigator("u1", "item-1", "editor", "gpt-5-mini", false)
	k5 := cacheKeyItemNavigator("u1", "item-1", "editor", "gpt-5", true)

	if k1 == k2 || k1 == k3 || k1 == k4 || k1 == k5 {
		t.Fatalf("cache key should vary, got k1=%q k2=%q k3=%q k4=%q k5=%q", k1, k2, k3, k4, k5)
	}
}
