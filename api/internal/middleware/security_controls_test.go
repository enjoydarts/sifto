package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLLMGeneratingGETsUseLLMTier(t *testing.T) {
	for _, path := range []string{"/api/items/123/navigator", "/api/sources/recommended", "/api/sources/suggestions", "/api/sources/navigator", "/api/briefing/navigator"} {
		if tier := rateLimitTierForRequest(httptest.NewRequest("GET", path, nil)); tier == nil || tier.Name != TierLLM.Name {
			t.Errorf("%s tier = %v, want llm", path, tier)
		}
	}
}

func TestPublicCacheBustIsIgnored(t *testing.T) {
	t.Setenv("INNGEST_DEV", "false")
	t.Setenv("ALLOW_DEV_AUTH_BYPASS", "false")
	handler := IgnoreClientCacheBust(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cache_bust") != "" || r.URL.Query().Get("days") != "30" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/llm-usage/summary?cache_bust=1&days=30", nil))
}
