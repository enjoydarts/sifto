package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimitTier struct {
	Name   string
	Limit  int
	Window time.Duration
}

var (
	TierLLM   = RateLimitTier{Name: "llm", Limit: 10, Window: time.Minute}
	TierWrite = RateLimitTier{Name: "write", Limit: 30, Window: time.Minute}
	TierRead  = RateLimitTier{Name: "read", Limit: 120, Window: time.Minute}
)

type RateLimiter struct {
	client *redis.Client
	prefix string
	now    func() time.Time
}

func NewRateLimiter(client *redis.Client, prefix string) *RateLimiter {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "sifto"
	}
	return &RateLimiter{
		client: client,
		prefix: prefix,
		now:    time.Now,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tier := rateLimitTierForRequest(r)
		if tier == nil {
			next.ServeHTTP(w, r)
			return
		}

		userID := GetUserID(r)
		if userID == "" || rl == nil || rl.client == nil {
			next.ServeHTTP(w, r)
			return
		}

		allowed, remaining, retryAfter, err := rl.check(r.Context(), *tier, userID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		appendExposeHeaders(w.Header(), "X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After")
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(tier.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func rateLimitTierForRequest(r *http.Request) *RateLimitTier {
	if r == nil {
		return nil
	}
	if r.Method == http.MethodPost && r.URL != nil && r.URL.Path == "/api/ask" {
		return &TierLLM
	}
	switch r.Method {
	case http.MethodGet:
		return &TierRead
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return &TierWrite
	default:
		return nil
	}
}

func (rl *RateLimiter) check(ctx context.Context, tier RateLimitTier, userID string) (allowed bool, remaining int, retryAfter int, err error) {
	if rl == nil || rl.client == nil {
		return true, tier.Limit, 0, nil
	}
	nowFn := rl.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()
	windowSec := int64(tier.Window / time.Second)
	if windowSec <= 0 {
		windowSec = 60
	}

	currentWindow := now.Unix() / windowSec * windowSec
	prevWindow := currentWindow - windowSec
	elapsed := float64(now.Unix()-currentWindow) / float64(windowSec)

	currentKey := rl.key(fmt.Sprintf("ratelimit:%s:%s:%d", tier.Name, userID, currentWindow))
	prevKey := rl.key(fmt.Sprintf("ratelimit:%s:%s:%d", tier.Name, userID, prevWindow))

	pipe := rl.client.Pipeline()
	prevCmd := pipe.Get(ctx, prevKey)
	currentCmd := pipe.Get(ctx, currentKey)
	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return true, tier.Limit, 0, err
	}

	prevCount := 0
	if v, getErr := prevCmd.Int(); getErr == nil {
		prevCount = v
	}
	currentCount := 0
	if v, getErr := currentCmd.Int(); getErr == nil {
		currentCount = v
	}

	estimated := float64(prevCount)*(1-elapsed) + float64(currentCount)
	if estimated >= float64(tier.Limit) {
		retryAfter = int(float64(windowSec) * (1 - elapsed))
		if retryAfter < 1 {
			retryAfter = 1
		}
		return false, 0, retryAfter, nil
	}

	writePipe := rl.client.Pipeline()
	writePipe.Incr(ctx, currentKey)
	writePipe.Expire(ctx, currentKey, tier.Window*2)
	if _, err := writePipe.Exec(ctx); err != nil {
		return true, tier.Limit, 0, err
	}

	remaining = tier.Limit - int(estimated) - 1
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining, 0, nil
}

func (rl *RateLimiter) key(value string) string {
	if rl == nil || rl.prefix == "" {
		return value
	}
	return rl.prefix + ":" + value
}

func appendExposeHeaders(headers http.Header, values ...string) {
	if headers == nil || len(values) == 0 {
		return
	}
	existing := headers.Values("Access-Control-Expose-Headers")
	seen := make(map[string]struct{}, len(existing)+len(values))
	parts := make([]string, 0, len(existing)+len(values))
	for _, line := range existing {
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			parts = append(parts, part)
		}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
	}
	headers.Set("Access-Control-Expose-Headers", strings.Join(parts, ", "))
}
