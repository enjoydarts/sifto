package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/enjoydarts/sifto/api/internal/middleware"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	ctx := context.Background()
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
			Release:          os.Getenv("APP_COMMIT_SHA"),
			AttachStacktrace: true,
		}); err != nil {
			log.Printf("sentry init error: %v", err)
		} else {
			defer sentry.Flush(2 * time.Second)
		}
	}

	deps, _, err := initDeps(ctx)
	if err != nil {
		log.Fatalf("init deps: %v", err)
	}
	defer deps.db.Close()

	preloadDynamicModels(ctx, deps)

	redisClient, redisPrefix := service.RedisClientFromCache(deps.cache)
	rateLimiter := middleware.NewRateLimiter(redisClient, redisPrefix)

	modules := []appModule{
		buildInternalModule(deps),
		buildItemsModule(deps),
		buildSourcesModule(deps),
		buildSettingsModule(deps),
		buildAudioBriefingModule(deps),
		buildLLMModelsModule(deps),
		buildAskModule(deps),
		buildDigestModule(deps),
		buildLLMUsageModule(deps),
		buildDashboardModule(deps),
		buildReviewsModule(deps),
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	registerHealthEndpoint(r)

	for _, m := range modules {
		if m.registerPublic != nil {
			m.registerPublic(r)
		}
	}

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(repository.NewUserIdentityRepo(deps.db), deps.clerkVerifier))
		r.Use(rateLimiter.Middleware)

		for _, m := range modules {
			if m.registerAPI != nil {
				m.registerAPI(r)
			}
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	commitSHA := os.Getenv("APP_COMMIT_SHA")
	if commitSHA == "" {
		commitSHA = "unknown"
	}

	log.Printf("api listening on :%s", port)
	log.Printf("api build commit=%s", commitSHA)
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	if err := http.ListenAndServe(":"+port, sentryHandler.Handle(r)); err != nil {
		log.Fatal(err)
	}
}
