package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/handler"
	inngestfn "github.com/enjoydarts/sifto/api/internal/inngest"
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

	db, err := repository.NewPool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	worker := service.NewWorkerClient()
	openAI := service.NewOpenAIClient()
	resend := service.NewResendClient()
	oneSignal := service.NewOneSignalClient()
	secretCipher := service.NewSecretCipher()
	clerkVerifier := service.NewClerkTokenVerifierFromEnv()
	githubApp := service.NewGitHubAppClientFromEnv()
	cache, err := service.NewJSONCacheFromEnv()
	if err != nil {
		log.Fatalf("json cache: %v", err)
	}
	eventPublisher, err := service.NewEventPublisher()
	if err != nil {
		log.Fatalf("event publisher: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	userIdentityRepo := repository.NewUserIdentityRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	readingGoalRepo := repository.NewReadingGoalRepo(db)
	reviewQueueRepo := repository.NewReviewQueueRepo(db)
	askInsightRepo := repository.NewAskInsightRepo(db)
	weeklyReviewRepo := repository.NewWeeklyReviewRepo(db)
	sourceOptimizationRepo := repository.NewSourceOptimizationRepo(db)
	notificationPriorityRepo := repository.NewNotificationPriorityRepo(db)
	obsidianExportRepo := repository.NewObsidianExportRepo(db)
	itemExportRepo := repository.NewItemExportRepo(db)
	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)
	itemInngestRepo := repository.NewItemInngestRepo(db)
	digestRepo := repository.NewDigestRepo(db)
	digestInngestRepo := repository.NewDigestInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	llmValueMetricsRepo := repository.NewLLMValueMetricsRepo(db)
	llmExecutionRepo := repository.NewLLMExecutionEventRepo(db)
	providerModelUpdateRepo := repository.NewProviderModelUpdateRepo(db)
	briefingSnapshotRepo := repository.NewBriefingSnapshotRepo(db)
	streakRepo := repository.NewReadingStreakRepo(db)
	prefProfileRepo := repository.NewPreferenceProfileRepo(db)
	obsidianExportSvc := service.NewObsidianExportService(itemRepo, itemExportRepo, obsidianExportRepo, githubApp)
	settingsH := handler.NewSettingsHandler(userSettingsRepo, obsidianExportRepo, notificationPriorityRepo, llmUsageRepo, secretCipher, githubApp, obsidianExportSvc, cache)
	readingGoalsH := handler.NewReadingGoalsHandler(readingGoalRepo)
	itemNotesH := handler.NewItemNotesHandler(itemRepo, reviewQueueRepo)
	reviewsH := handler.NewReviewsHandler(reviewQueueRepo, weeklyReviewRepo)
	askInsightsH := handler.NewAskInsightsHandler(askInsightRepo)
	providerModelUpdateH := handler.NewProviderModelUpdateHandler(providerModelUpdateRepo)

	internalH := handler.NewInternalHandler(userRepo, userIdentityRepo, obsidianExportRepo, itemInngestRepo, digestInngestRepo, userSettingsRepo, secretCipher, eventPublisher, db, cache, worker, oneSignal, githubApp)
	sourceH := handler.NewSourceHandler(sourceRepo, itemRepo, sourceOptimizationRepo, userSettingsRepo, llmUsageRepo, worker, secretCipher, eventPublisher, cache)
	itemH := handler.NewItemHandler(itemRepo, sourceRepo, readingGoalRepo, streakRepo, briefingSnapshotRepo, prefProfileRepo, reviewQueueRepo, eventPublisher, cache)
	digestH := handler.NewDigestHandler(digestRepo)
	llmUsageH := handler.NewLLMUsageHandlerWithValueMetrics(llmUsageRepo, llmExecutionRepo, llmValueMetricsRepo, cache)
	dashboardH := handler.NewDashboardHandler(sourceRepo, itemRepo, digestRepo, llmUsageRepo, cache)
	briefingH := handler.NewBriefingHandler(itemRepo, briefingSnapshotRepo, streakRepo, cache)
	askH := handler.NewAskHandler(itemRepo, userSettingsRepo, llmUsageRepo, secretCipher, worker, openAI, cache)

	inngestHandler := inngestfn.NewHandler(db, worker, resend, oneSignal, obsidianExportSvc, cache)

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
	r.Mount("/api/inngest", logInngestRequests(inngestHandler))

	// Next.js からのみ呼ばれる内部エンドポイント（X-Internal-Secret で保護）
	r.Post("/api/internal/users/upsert", internalH.UpsertUser)
	r.Post("/api/internal/users/resolve-identity", internalH.ResolveIdentity)
	r.Post("/api/internal/settings/obsidian-github/installation", internalH.UpsertObsidianGitHubInstallation)
	r.Post("/api/internal/debug/digests/generate", internalH.DebugGenerateDigest)
	r.Post("/api/internal/debug/digests/send", internalH.DebugSendDigest)
	r.Post("/api/internal/debug/embeddings/backfill", internalH.DebugBackfillEmbeddings)
	r.Post("/api/internal/debug/titles/backfill", internalH.DebugBackfillTranslatedTitles)
	r.Post("/api/internal/debug/push/test", internalH.DebugSendPushTest)
	r.Get("/api/internal/debug/system-status", internalH.DebugSystemStatus)

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(userIdentityRepo, clerkVerifier))

		r.Route("/sources", func(r chi.Router) {
			r.Get("/", sourceH.List)
			r.Get("/opml", sourceH.ExportOPML)
			r.Post("/opml/import", sourceH.ImportOPML)
			r.Post("/inoreader/import", sourceH.ImportInoreader)
			r.Get("/health", sourceH.Health)
			r.Get("/optimization", sourceH.Optimization)
			r.Get("/recommended", sourceH.Recommended)
			r.Post("/", sourceH.Create)
			r.Post("/discover", sourceH.Discover)
			r.Get("/suggestions", sourceH.Suggest)
			r.Patch("/{id}", sourceH.Update)
			r.Delete("/{id}", sourceH.Delete)
		})

		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemH.List)
			r.Get("/favorites/export-markdown", itemH.ExportFavoritesMarkdown)
			r.Get("/stats", itemH.Stats)
			r.Get("/ux-metrics", itemH.UXMetrics)
			r.Get("/topic-trends", itemH.TopicTrends)
			r.Post("/retry-failed", itemH.RetryFailed)
			r.Get("/reading-plan", itemH.ReadingPlan)
			r.Get("/focus-queue", itemH.FocusQueue)
			r.Get("/today-queue", itemH.TodayQueue)
			r.Get("/triage-all", itemH.TriageAll)
			r.Get("/{id}/related", itemH.Related)
			r.Put("/{id}/note", func(w http.ResponseWriter, r *http.Request) {
				itemNotesH.UpsertNote(w, r, chi.URLParam(r, "id"))
			})
			r.Get("/{id}/highlights", func(w http.ResponseWriter, r *http.Request) {
				itemNotesH.ListHighlights(w, r, chi.URLParam(r, "id"))
			})
			r.Post("/{id}/highlights", func(w http.ResponseWriter, r *http.Request) {
				itemNotesH.CreateHighlight(w, r, chi.URLParam(r, "id"))
			})
			r.Delete("/{id}/highlights/{highlightId}", func(w http.ResponseWriter, r *http.Request) {
				itemNotesH.DeleteHighlight(w, r, chi.URLParam(r, "id"), chi.URLParam(r, "highlightId"))
			})
			r.Delete("/{id}", itemH.Delete)
			r.Get("/{id}", itemH.GetDetail)
			r.Patch("/{id}/feedback", itemH.SetFeedback)
			r.Post("/{id}/read", itemH.MarkRead)
			r.Post("/mark-read-bulk", itemH.MarkReadBulk)
			r.Post("/mark-later-bulk", itemH.MarkLaterBulk)
			r.Delete("/{id}/read", itemH.MarkUnread)
			r.Post("/{id}/later", itemH.MarkLater)
			r.Delete("/{id}/later", itemH.UnmarkLater)
			r.Post("/{id}/retry", itemH.Retry)
			r.Post("/{id}/retry-from-facts", itemH.RetryFromFacts)
		})

		r.Route("/topics", func(r chi.Router) {
			r.Get("/pulse", itemH.TopicPulse)
		})

		r.Post("/ask", askH.Ask)
		r.Get("/ask/insights", askInsightsH.ListRecent)
		r.Post("/ask/insights", askInsightsH.Save)
		r.Delete("/ask/insights/{id}", func(w http.ResponseWriter, r *http.Request) {
			askInsightsH.Delete(w, r, chi.URLParam(r, "id"))
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
			r.Get("/analysis", llmUsageH.AnalysisSummary)
			r.Get("/current-month/by-provider", llmUsageH.ProviderSummaryCurrentMonth)
			r.Get("/current-month/by-purpose", llmUsageH.PurposeSummaryCurrentMonth)
			r.Get("/current-month/execution-summary", llmUsageH.ExecutionSummaryCurrentMonth)
			r.Get("/current-month/value-metrics", llmUsageH.ValueMetricsCurrentMonth)
		})

		r.Route("/provider-model-updates", func(r chi.Router) {
			r.Get("/", providerModelUpdateH.ListRecent)
		})

		r.Get("/briefing/today", briefingH.Today)
		r.Get("/dashboard", dashboardH.Get)
		r.Route("/reviews", func(r chi.Router) {
			r.Get("/due", reviewsH.Due)
			r.Post("/{id}/done", func(w http.ResponseWriter, r *http.Request) {
				reviewsH.MarkDone(w, r, chi.URLParam(r, "id"))
			})
			r.Post("/{id}/snooze", func(w http.ResponseWriter, r *http.Request) {
				reviewsH.Snooze(w, r, chi.URLParam(r, "id"))
			})
			r.Get("/weekly/latest", reviewsH.WeeklyLatest)
		})

		r.Route("/settings", func(r chi.Router) {
			r.Get("/", settingsH.Get)
			r.Get("/reading-goals", readingGoalsH.List)
			r.Post("/reading-goals", readingGoalsH.Create)
			r.Patch("/reading-goals/{id}", readingGoalsH.Update)
			r.Post("/reading-goals/{id}/archive", readingGoalsH.Archive)
			r.Post("/reading-goals/{id}/restore", readingGoalsH.Restore)
			r.Delete("/reading-goals/{id}", readingGoalsH.Delete)
			r.Get("/llm-catalog", settingsH.GetLLMCatalog)
			r.Patch("/", settingsH.UpdateBudget)
			r.Patch("/reading-plan", settingsH.UpdateReadingPlan)
			r.Patch("/notification-priority", settingsH.UpdateNotificationPriority)
			r.Patch("/llm-models", settingsH.UpdateLLMModels)
			r.Patch("/obsidian-export", settingsH.UpdateObsidianExport)
			r.Post("/obsidian-export/run", settingsH.RunObsidianExport)
			r.Get("/inoreader/connect", settingsH.InoreaderConnect)
			r.Get("/inoreader/callback", settingsH.InoreaderCallback)
			r.Delete("/inoreader-oauth", settingsH.DeleteInoreaderOAuth)
			r.Post("/anthropic-key", settingsH.SetAnthropicAPIKey)
			r.Delete("/anthropic-key", settingsH.DeleteAnthropicAPIKey)
			r.Post("/openai-key", settingsH.SetOpenAIAPIKey)
			r.Delete("/openai-key", settingsH.DeleteOpenAIAPIKey)
			r.Post("/google-key", settingsH.SetGoogleAPIKey)
			r.Delete("/google-key", settingsH.DeleteGoogleAPIKey)
			r.Post("/groq-key", settingsH.SetGroqAPIKey)
			r.Delete("/groq-key", settingsH.DeleteGroqAPIKey)
			r.Post("/deepseek-key", settingsH.SetDeepSeekAPIKey)
			r.Delete("/deepseek-key", settingsH.DeleteDeepSeekAPIKey)
			r.Post("/alibaba-key", settingsH.SetAlibabaAPIKey)
			r.Delete("/alibaba-key", settingsH.DeleteAlibabaAPIKey)
			r.Post("/mistral-key", settingsH.SetMistralAPIKey)
			r.Delete("/mistral-key", settingsH.DeleteMistralAPIKey)
			r.Post("/xai-key", settingsH.SetXAIAPIKey)
			r.Delete("/xai-key", settingsH.DeleteXAIAPIKey)
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
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	if err := http.ListenAndServe(":"+port, sentryHandler.Handle(r)); err != nil {
		log.Fatal(err)
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func logInngestRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("inngest request start method=%s path=%s remote=%s ua=%q", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
		if r.Method == http.MethodPut {
			log.Printf(
				"inngest request meta method=%s path=%s content_type=%q content_length=%d transfer_encoding=%q headers=%s",
				r.Method,
				r.URL.Path,
				r.Header.Get("Content-Type"),
				r.ContentLength,
				strings.Join(r.TransferEncoding, ","),
				formatInngestHeaders(r.Header),
			)
		}

		lrw := &loggingResponseWriter{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("inngest request panic method=%s path=%s remote=%s err=%v", r.Method, r.URL.Path, r.RemoteAddr, rec)
				panic(rec)
			}
			status := lrw.status
			if status == 0 {
				status = -1
			}
			log.Printf(
				"inngest request done method=%s path=%s remote=%s status=%d duration_ms=%d wrote_header=%t",
				r.Method,
				r.URL.Path,
				r.RemoteAddr,
				status,
				time.Since(start).Milliseconds(),
				lrw.wroteHeader,
			)
		}()

		next.ServeHTTP(lrw, r)
	})
}

func formatInngestHeaders(header http.Header) string {
	keys := []string{
		"Authorization",
		"Content-Type",
		"Content-Length",
		"X-Inngest-Signature",
		"X-Inngest-Env",
		"X-Inngest-Framework",
		"X-Inngest-Expected-Server-Kind",
		"X-Inngest-Server-Kind",
		"X-Inngest-SDK",
		"X-Inngest-Req-Version",
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		if strings.EqualFold(key, "Authorization") {
			parts = append(parts, key+"=<redacted>")
			continue
		}
		parts = append(parts, key+"="+value)
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " ")
}
