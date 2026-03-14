package handler

import (
	"context"
	"testing"
)

func TestSettingsCacheKeyUsesVersion(t *testing.T) {
	cache := newTestJSONCache()
	cache.versions[cacheVersionKeyUserSettings("u1")] = 9
	handler := &SettingsHandler{cache: cache}

	key, err := handler.settingsCacheKey(context.Background(), "u1")
	if err != nil {
		t.Fatalf("settingsCacheKey returned error: %v", err)
	}
	want := "v1:settings:get:u1:v=9"
	if key != want {
		t.Fatalf("settingsCacheKey = %q, want %q", key, want)
	}
}

func TestBumpUserSettingsVersion(t *testing.T) {
	cache := newTestJSONCache()
	handler := &SettingsHandler{cache: cache}

	if err := handler.bumpUserSettingsVersion(context.Background(), "u1"); err != nil {
		t.Fatalf("first bumpUserSettingsVersion returned error: %v", err)
	}
	if err := handler.bumpUserSettingsVersion(context.Background(), "u1"); err != nil {
		t.Fatalf("second bumpUserSettingsVersion returned error: %v", err)
	}

	if got, want := cache.versions[cacheVersionKeyUserSettings("u1")], int64(2); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
}
