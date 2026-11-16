package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
)

func dependencyHealthChecks(deps *appDeps) map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"db": func(ctx context.Context) error {
			if deps.db == nil {
				return fmt.Errorf("db not configured")
			}
			return deps.db.Ping(ctx)
		},
		"redis": func(ctx context.Context) error {
			client, _ := service.RedisClientFromCache(deps.cache)
			if client == nil {
				return fmt.Errorf("redis not configured")
			}
			return client.Ping(ctx).Err()
		},
		"worker":      deps.worker.Health,
		"meilisearch": deps.search.Health,
		"inngest":     service.InngestHealth,
	}
}

func registerHealthEndpoint(router chi.Router, checks map[string]func(context.Context) error) {
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		type result struct {
			name string
			err  error
		}
		results := make(chan result, len(checks))
		statuses := make(map[string]string, len(checks))
		for name, check := range checks {
			statuses[name] = "error"
			go func() { results <- result{name, check(ctx)} }()
		}
		for range checks {
			select {
			case res := <-results:
				if res.err == nil {
					statuses[res.name] = "ok"
				}
			case <-ctx.Done():
				// Unfinished checks stay failed; a stalled dependency must never
				// hold the HTTP response indefinitely.
			}
		}
		status, code := "ok", http.StatusOK
		for _, state := range statuses {
			if state != "ok" {
				status, code = "degraded", http.StatusServiceUnavailable
			}
		}
		commit := os.Getenv("APP_COMMIT_SHA")
		if commit == "" {
			commit = "unknown"
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "commit": commit, "checks": statuses})
	})
}
