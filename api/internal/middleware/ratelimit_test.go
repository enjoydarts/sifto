package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func newTestRateLimiter(t *testing.T) (*RateLimiter, *redis.Client) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "redis:6379", DB: 14})
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB failed: %v", err)
	}
	limiter := NewRateLimiter(client, "sifto-test")
	return limiter, client
}

func TestRateLimiterTierForRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/ask", nil)
	if tier := rateLimitTierForRequest(req); tier == nil || tier.Name != TierLLM.Name {
		t.Fatalf("POST /api/ask tier = %#v, want llm", tier)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/items", nil)
	if tier := rateLimitTierForRequest(req); tier == nil || tier.Name != TierRead.Name {
		t.Fatalf("GET /api/items tier = %#v, want read", tier)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/settings", nil)
	if tier := rateLimitTierForRequest(req); tier == nil || tier.Name != TierWrite.Name {
		t.Fatalf("PATCH /api/settings tier = %#v, want write", tier)
	}
}

func TestRateLimiterBlocksAskAfterLimit(t *testing.T) {
	limiter, _ := newTestRateLimiter(t)
	limiter.now = func() time.Time {
		return time.Unix(1711100415, 0).UTC()
	}

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/ask", nil)
		req = req.WithContext(context.WithValue(req.Context(), UserIDKey, "u1"))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("initial request status = %d, want 200 body=%s", rr.Code, rr.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ask", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked request status = %d, want 429 body=%s", rr.Code, rr.Body.String())
	}
	if got, want := rr.Header().Get("X-RateLimit-Limit"), "10"; got != want {
		t.Fatalf("X-RateLimit-Limit = %q, want %q", got, want)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After should be set when limited")
	}
}

func TestRateLimiterFailsOpenWhenRedisUnavailable(t *testing.T) {
	limiter := NewRateLimiter(nil, "sifto-test")
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, "u1"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}
