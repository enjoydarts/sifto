package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	inngestfn "github.com/minoru-kitayama/sifto/api/internal/inngest"
	"github.com/minoru-kitayama/sifto/api/internal/handler"
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
	eventPublisher, err := service.NewEventPublisher()
	if err != nil {
		log.Fatalf("event publisher: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)
	itemInngestRepo := repository.NewItemInngestRepo(db)
	digestRepo := repository.NewDigestRepo(db)
	digestInngestRepo := repository.NewDigestInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)

	internalH := handler.NewInternalHandler(userRepo, itemInngestRepo, digestInngestRepo, eventPublisher)
	sourceH := handler.NewSourceHandler(sourceRepo, itemRepo, eventPublisher)
	itemH := handler.NewItemHandler(itemRepo, eventPublisher)
	digestH := handler.NewDigestHandler(digestRepo)
	llmUsageH := handler.NewLLMUsageHandler(llmUsageRepo)

	inngestHandler := inngestfn.NewHandler(db, worker, resend)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// Inngest serve endpoint（認証不要）
	r.Mount("/api/inngest", inngestHandler)

	// NextAuth からのみ呼ばれる内部エンドポイント（X-Internal-Secret で保護）
	r.Post("/api/internal/users/upsert", internalH.UpsertUser)
	r.Post("/api/internal/debug/digests/generate", internalH.DebugGenerateDigest)
	r.Post("/api/internal/debug/digests/send", internalH.DebugSendDigest)

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth)

		r.Route("/sources", func(r chi.Router) {
			r.Get("/", sourceH.List)
			r.Post("/", sourceH.Create)
			r.Post("/discover", sourceH.Discover)
			r.Patch("/{id}", sourceH.Update)
			r.Delete("/{id}", sourceH.Delete)
		})

		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemH.List)
			r.Post("/retry-failed", itemH.RetryFailed)
			r.Get("/{id}", itemH.GetDetail)
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
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
