package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/handler"
	inngestfn "github.com/minoru-kitayama/sifto/api/internal/inngest"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
)

func main() {
	ctx := context.Background()

	db, err := repository.NewPool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	worker := service.NewWorkerClient()
	resend := service.NewResendClient()
	secretCipher := service.NewSecretCipher()
	cache, err := service.NewJSONCacheFromEnv()
	if err != nil {
		log.Fatalf("json cache: %v", err)
	}
	eventPublisher, err := service.NewEventPublisher()
	if err != nil {
		log.Fatalf("event publisher: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)
	itemInngestRepo := repository.NewItemInngestRepo(db)
	digestRepo := repository.NewDigestRepo(db)
	digestInngestRepo := repository.NewDigestInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	settingsH := handler.NewSettingsHandler(userSettingsRepo, llmUsageRepo, secretCipher)

	internalH := handler.NewInternalHandler(userRepo, itemInngestRepo, digestInngestRepo, eventPublisher)
	sourceH := handler.NewSourceHandler(sourceRepo, itemRepo, userSettingsRepo, llmUsageRepo, worker, secretCipher, eventPublisher)
	itemH := handler.NewItemHandler(itemRepo, eventPublisher, cache)
	digestH := handler.NewDigestHandler(digestRepo)
	llmUsageH := handler.NewLLMUsageHandler(llmUsageRepo)
	dashboardH := handler.NewDashboardHandler(sourceRepo, itemRepo, digestRepo, llmUsageRepo, cache)

	inngestHandler := inngestfn.NewHandler(db, worker, resend)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		commitSHA := os.Getenv("APP_COMMIT_SHA")
		if commitSHA == "" {
			commitSHA = "unknown"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"commit": commitSHA,
		})
	})

	// Inngest serve endpoint（認証不要）
	r.Mount("/api/inngest", inngestHandler)

	// NextAuth からのみ呼ばれる内部エンドポイント（X-Internal-Secret で保護）
	r.Post("/api/internal/users/upsert", internalH.UpsertUser)
	r.Post("/api/internal/debug/digests/generate", internalH.DebugGenerateDigest)
	r.Post("/api/internal/debug/digests/send", internalH.DebugSendDigest)
	r.Post("/api/internal/debug/embeddings/backfill", internalH.DebugBackfillEmbeddings)

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth)

		r.Route("/sources", func(r chi.Router) {
			r.Get("/", sourceH.List)
			r.Post("/", sourceH.Create)
			r.Post("/discover", sourceH.Discover)
			r.Get("/suggestions", sourceH.Suggest)
			r.Patch("/{id}", sourceH.Update)
			r.Delete("/{id}", sourceH.Delete)
		})

		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemH.List)
			r.Get("/stats", itemH.Stats)
			r.Get("/topic-trends", itemH.TopicTrends)
			r.Post("/retry-failed", itemH.RetryFailed)
			r.Get("/reading-plan", itemH.ReadingPlan)
			r.Get("/{id}/related", itemH.Related)
			r.Get("/{id}", itemH.GetDetail)
			r.Patch("/{id}/feedback", itemH.SetFeedback)
			r.Post("/{id}/read", itemH.MarkRead)
			r.Delete("/{id}/read", itemH.MarkUnread)
			r.Post("/{id}/retry", itemH.Retry)
		})

		r.Route("/digests", func(r chi.Router) {
			r.Get("/", digestH.List)
			r.Get("/latest", digestH.GetLatest)
			r.Get("/{id}", digestH.GetDetail)
		})

		r.Route("/llm-usage", func(r chi.Router) {
			r.Get("/", llmUsageH.List)
			r.Get("/summary", llmUsageH.DailySummary)
			r.Get("/by-model", llmUsageH.ModelSummary)
		})

		r.Get("/dashboard", dashboardH.Get)

		r.Route("/settings", func(r chi.Router) {
			r.Get("/", settingsH.Get)
			r.Patch("/", settingsH.UpdateBudget)
			r.Patch("/reading-plan", settingsH.UpdateReadingPlan)
			r.Patch("/llm-models", settingsH.UpdateLLMModels)
			r.Post("/anthropic-key", settingsH.SetAnthropicAPIKey)
			r.Delete("/anthropic-key", settingsH.DeleteAnthropicAPIKey)
			r.Post("/openai-key", settingsH.SetOpenAIAPIKey)
			r.Delete("/openai-key", settingsH.DeleteOpenAIAPIKey)
		})
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
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
