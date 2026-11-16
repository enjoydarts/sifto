package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHealthReportsInngestFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
		want int
	}{
		{"healthy", 200, `{"status":200,"message":"OK"}`, 200},
		{"upstream failure", 502, `bad gateway`, 503},
		{"login page", 200, `<html>login</html>`, 503},
		{"unhealthy payload", 200, `{"status":503}`, 503},
		{"redirect", 302, ``, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("unexpected upstream path: %s", r.URL.Path)
				}
				if r.Header.Get("CF-Access-Client-Secret") != "private-test-secret" {
					t.Error("missing upstream Access authentication")
				}
				w.Header().Set("Location", "/login")
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			t.Setenv("INNGEST_BASE_URL", upstream.URL)
			t.Setenv("INNGEST_CF_ACCESS_CLIENT_ID", "test-id")
			t.Setenv("INNGEST_CF_ACCESS_CLIENT_SECRET", "private-test-secret")
			t.Setenv("APP_COMMIT_SHA", "test-commit")
			assertHealthResponse(t, tc.want)
		})
	}
}

func TestHealthReportsDisconnectedInngest(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	t.Setenv("INNGEST_BASE_URL", upstream.URL)
	assertHealthResponse(t, http.StatusServiceUnavailable)
}

func TestHealthWithoutSelfHostedInngest(t *testing.T) {
	t.Setenv("INNGEST_BASE_URL", "")
	assertHealthResponse(t, http.StatusOK)
}

func TestHealthRejectsUnconfiguredRedis(t *testing.T) {
	checks := dependencyHealthChecks(&appDeps{cache: service.NoopJSONCache{}})
	if err := checks["redis"](context.Background()); err == nil {
		t.Fatal("Redis must not be reported healthy by a no-op cache")
	}
}

func TestHealthChecksEveryDependency(t *testing.T) {
	for _, failed := range []string{"", "db", "redis", "worker", "meilisearch", "inngest"} {
		t.Run("failed="+failed, func(t *testing.T) {
			checks := map[string]func(context.Context) error{}
			for _, name := range []string{"db", "redis", "worker", "meilisearch", "inngest"} {
				checks[name] = func(ctx context.Context) error {
					if _, ok := ctx.Deadline(); !ok {
						t.Error("dependency check must have a deadline")
					}
					if name == failed {
						return errors.New("private connection credentials")
					}
					return nil
				}
			}
			router := chi.NewRouter()
			registerHealthEndpoint(router, checks)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
			want := http.StatusOK
			if failed != "" {
				want = http.StatusServiceUnavailable
			}
			if response.Code != want {
				t.Fatalf("status = %d, want %d: %s", response.Code, want, response.Body)
			}
			var body struct {
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Checks) != len(checks) {
				t.Fatalf("missing dependency results: %s", response.Body)
			}
			for name := range checks {
				wantStatus := "ok"
				if name == failed {
					wantStatus = "error"
				}
				if body.Checks[name] != wantStatus {
					t.Errorf("%s = %s, want %s", name, body.Checks[name], wantStatus)
				}
			}
			if strings.Contains(response.Body.String(), "private connection") {
				t.Fatal("leaked credentials")
			}
		})
	}
}

func assertHealthResponse(t *testing.T, want int) {
	t.Helper()
	router := chi.NewRouter()
	registerHealthEndpoint(router, map[string]func(context.Context) error{"inngest": service.InngestHealth})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != want {
		t.Fatalf("health status = %d, want %d; body=%s", response.Code, want, response.Body)
	}
	var body struct {
		Status string `json:"status"`
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if (body.Status == "ok") != (want == http.StatusOK) {
		t.Fatalf("unexpected health status: %q", body.Status)
	}
	if strings.Contains(response.Body.String(), "private-test-secret") || strings.Contains(response.Body.String(), "/login") {
		t.Fatal("health response leaks upstream details")
	}
}

func TestHealthRejectsStalledDependency(t *testing.T) {
	router := chi.NewRouter()
	registerHealthEndpoint(router, map[string]func(context.Context) error{
		"db": func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
	})
	response := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusServiceUnavailable || time.Since(start) > 4*time.Second {
		t.Fatalf("stalled dependency: code=%d elapsed=%s", response.Code, time.Since(start))
	}
}

func TestHealthDetectsExhaustedApplicationDBPool(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	router := chi.NewRouter()
	registerHealthEndpoint(router, map[string]func(context.Context) error{"db": dependencyHealthChecks(&appDeps{db: pool})["db"]})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("exhausted pool: %d %s", response.Code, response.Body)
	}
	conn.Release()
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("recovered pool: %d %s", response.Code, response.Body)
	}
}
