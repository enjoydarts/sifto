package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
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
	search, err := service.NewMeilisearchServiceFromEnv()
	if err != nil {
		log.Fatalf("meilisearch: %v", err)
	}
	redisClient, redisPrefix := service.RedisClientFromCache(cache)
	eventPublisher, err := service.NewEventPublisher()
	if err != nil {
		log.Fatalf("event publisher: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	userIdentityRepo := repository.NewUserIdentityRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	readingGoalRepo := repository.NewReadingGoalRepo(db)
	reviewQueueRepo := repository.NewReviewQueueRepo(db)
	askInsightRepo := repository.NewAskInsightRepo(db)
	weeklyReviewRepo := repository.NewWeeklyReviewRepo(db)
	sourceOptimizationRepo := repository.NewSourceOptimizationRepo(db)
	notificationPriorityRepo := repository.NewNotificationPriorityRepo(db)
	pushNotificationLogRepo := repository.NewPushNotificationLogRepo(db)
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
	providerModelSnapshotSyncSvc := service.NewProviderModelSnapshotSyncService(
		userRepo,
		userSettingsRepo,
		providerModelUpdateRepo,
		pushNotificationLogRepo,
		oneSignal,
		secretCipher,
	)
	openRouterModelRepo := repository.NewOpenRouterModelRepo(db)
	poeModelRepo := repository.NewPoeModelRepo(db)
	aivisModelRepo := repository.NewAivisModelRepo(db)
	poeUsageRepo := repository.NewPoeUsageRepo(db)
	openRouterModelOverrideRepo := repository.NewOpenRouterModelOverrideRepo(db)
	promptTemplateRepo := repository.NewPromptTemplateRepo(db)
	promptResolver := service.NewPromptResolver(promptTemplateRepo)
	briefingSnapshotRepo := repository.NewBriefingSnapshotRepo(db)
	aiNavigatorBriefRepo := repository.NewAINavigatorBriefRepo(db)
	playbackSessionRepo := repository.NewPlaybackSessionRepo(db)
	streakRepo := repository.NewReadingStreakRepo(db)
	prefProfileRepo := repository.NewPreferenceProfileRepo(db)
	obsidianExportSvc := service.NewObsidianExportService(itemRepo, itemExportRepo, obsidianExportRepo, githubApp)
	promptAdminAuth := service.NewPromptAdminAuthServiceFromEnv()
	settingsH := handler.NewSettingsHandler(userSettingsRepo, audioBriefingRepo, aivisModelRepo, obsidianExportRepo, notificationPriorityRepo, prefProfileRepo, llmUsageRepo, openRouterModelOverrideRepo, secretCipher, githubApp, obsidianExportSvc, worker, cache)
	promptAdminH := handler.NewPromptAdminHandler(promptTemplateRepo, promptAdminAuth, userRepo)
	audioConcatRunner := service.NewAudioConcatRunnerFromEnv()
	audioBriefingVoiceRunner := service.NewAudioBriefingVoiceRunner(audioBriefingRepo, userSettingsRepo, secretCipher, worker)
	audioBriefingConcatStarter := service.NewAudioBriefingConcatStarter(audioBriefingRepo, audioConcatRunner)
	audioBriefingOrchestrator := service.NewAudioBriefingOrchestrator(audioBriefingRepo, userSettingsRepo, llmUsageRepo, promptResolver, secretCipher, worker, cache, audioBriefingVoiceRunner, audioBriefingConcatStarter)
	audioBriefingDeleteSvc := service.NewAudioBriefingDeleteService(audioBriefingRepo, worker)
	audioBriefingPublishedNotifier := service.NewAudioBriefingPublishedNotifier(userRepo, notificationPriorityRepo, pushNotificationLogRepo, oneSignal, nil)
	podcastPublicationSvc := service.NewPodcastPublicationService(audioBriefingRepo, worker)
	audioBriefingsH := handler.NewAudioBriefingsHandler(audioBriefingRepo, audioBriefingOrchestrator, audioBriefingVoiceRunner, audioBriefingConcatStarter, audioBriefingDeleteSvc, eventPublisher, worker)
	summaryAudioPlayerSvc := service.NewSummaryAudioPlayerService(itemRepo, audioBriefingRepo, userSettingsRepo, secretCipher, worker)
	summaryAudioPlayerH := handler.NewSummaryAudioPlayerHandler(summaryAudioPlayerSvc)
	playbackSessionsSvc := service.NewPlaybackSessionsService(playbackSessionRepo)
	playbackSessionsH := handler.NewPlaybackSessionsHandler(playbackSessionsSvc)
	aiNavigatorBriefSvc := service.NewAINavigatorBriefService(aiNavigatorBriefRepo, itemRepo, userSettingsRepo, userRepo, pushNotificationLogRepo, llmUsageRepo, worker, secretCipher, oneSignal, eventPublisher, cache, nil)
	aiNavigatorBriefH := handler.NewAINavigatorBriefHandler(aiNavigatorBriefSvc)
	internalAudioBriefingsH := handler.NewInternalAudioBriefingsHandler(audioBriefingRepo, audioBriefingPublishedNotifier, podcastPublicationSvc)
	podcastFeedSvc := service.NewPodcastFeedService(userSettingsRepo, audioBriefingRepo, worker)
	podcastsH := handler.NewPodcastsHandler(podcastFeedSvc, cache)
	readingGoalsH := handler.NewReadingGoalsHandler(readingGoalRepo)
	itemNotesH := handler.NewItemNotesHandler(itemRepo, reviewQueueRepo, eventPublisher)
	reviewsH := handler.NewReviewsHandler(reviewQueueRepo, weeklyReviewRepo)
	askInsightsH := handler.NewAskInsightsHandler(askInsightRepo)
	providerModelUpdateH := handler.NewProviderModelUpdateHandler(providerModelUpdateRepo, providerModelSnapshotSyncSvc)
	openRouterCatalogSvc := service.NewOpenRouterCatalogService()
	openRouterModelsH := handler.NewOpenRouterModelsHandler(openRouterModelRepo, openRouterModelOverrideRepo, providerModelUpdateRepo, openRouterCatalogSvc, cache)
	poeCatalogSvc := service.NewPoeCatalogService()
	poeUsageSvc := service.NewPoeUsageService(poeUsageRepo)
	poeModelsH := handler.NewPoeModelsHandler(poeModelRepo, userSettingsRepo, secretCipher, providerModelUpdateRepo, poeCatalogSvc, poeUsageSvc)
	aivisCatalogSvc := service.NewAivisCatalogService()
	aivisModelsH := handler.NewAivisModelsHandler(aivisModelRepo, providerModelUpdateRepo, aivisCatalogSvc)

	internalH := handler.NewInternalHandler(userRepo, userIdentityRepo, obsidianExportRepo, itemInngestRepo, digestInngestRepo, userSettingsRepo, secretCipher, eventPublisher, db, cache, worker, oneSignal, githubApp, search)
	sourceH := handler.NewSourceHandler(sourceRepo, itemRepo, sourceOptimizationRepo, userSettingsRepo, llmUsageRepo, worker, secretCipher, eventPublisher, cache)
	itemH := handler.NewItemHandler(itemRepo, sourceRepo, readingGoalRepo, streakRepo, briefingSnapshotRepo, prefProfileRepo, reviewQueueRepo, userSettingsRepo, llmUsageRepo, eventPublisher, secretCipher, worker, cache, search)
	digestH := handler.NewDigestHandler(digestRepo)
	llmUsageH := handler.NewLLMUsageHandlerWithValueMetrics(llmUsageRepo, llmExecutionRepo, llmValueMetricsRepo, cache)
	dashboardH := handler.NewDashboardHandler(sourceRepo, itemRepo, digestRepo, llmUsageRepo, cache)
	briefingH := handler.NewBriefingHandler(itemRepo, briefingSnapshotRepo, streakRepo, userSettingsRepo, llmUsageRepo, secretCipher, worker, cache)
	askH := handler.NewAskHandler(itemRepo, userSettingsRepo, llmUsageRepo, secretCipher, worker, openAI, cache)
	rateLimiter := middleware.NewRateLimiter(redisClient, redisPrefix)

	if latestModels, _, err := openRouterModelRepo.ListLatestSnapshots(ctx); err != nil {
		log.Printf("openrouter snapshot preload failed: %v", err)
	} else {
		service.SetDynamicChatModelsForProvider("openrouter", service.OpenRouterSnapshotsToCatalogModels(latestModels))
	}
	if latestModels, _, err := poeModelRepo.ListLatestSnapshots(ctx); err != nil {
		log.Printf("poe snapshot preload failed: %v", err)
	} else {
		service.SetDynamicChatModelsForProvider("poe", service.PoeSnapshotsToCatalogModels(latestModels))
	}
	inngestHandler := inngestfn.NewHandler(db, worker, resend, oneSignal, obsidianExportSvc, cache, search)

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
	r.Get("/podcasts/{slug}/feed.xml", podcastsH.Feed)
	r.Head("/podcasts/{slug}/feed.xml", podcastsH.Feed)

	// Inngest serve endpoint（認証不要）
	r.Mount("/api/inngest", ensureInngestPutNoContent(inngestHandler))

	// Next.js からのみ呼ばれる内部エンドポイント（X-Internal-Secret で保護）
	r.Post("/api/internal/users/upsert", internalH.UpsertUser)
	r.Post("/api/internal/users/resolve-identity", internalH.ResolveIdentity)
	r.Post("/api/internal/settings/obsidian-github/installation", internalH.UpsertObsidianGitHubInstallation)
	r.Post("/api/internal/debug/digests/generate", internalH.DebugGenerateDigest)
	r.Post("/api/internal/debug/digests/send", internalH.DebugSendDigest)
	r.Post("/api/internal/debug/embeddings/backfill", internalH.DebugBackfillEmbeddings)
	r.Post("/api/internal/debug/titles/backfill", internalH.DebugBackfillTranslatedTitles)
	r.Post("/api/internal/debug/llm-usage/backfill-openrouter-costs", internalH.DebugBackfillOpenRouterCosts)
	r.Get("/api/internal/debug/search/backfill", internalH.DebugGetItemSearchBackfillRuns)
	r.Post("/api/internal/debug/search/backfill", internalH.DebugBackfillItemSearch)
	r.Delete("/api/internal/debug/search/backfill", internalH.DebugDeleteFinishedItemSearchBackfillRuns)
	r.Post("/api/internal/debug/push/test", internalH.DebugSendPushTest)
	r.Get("/api/internal/debug/system-status", internalH.DebugSystemStatus)
	r.Post("/api/internal/audio-briefings/{id}/concat-complete", internalAudioBriefingsH.ConcatComplete)
	r.Post("/api/internal/audio-briefings/chunks/{chunkID}/heartbeat", internalAudioBriefingsH.ChunkHeartbeat)

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(userIdentityRepo, clerkVerifier))
		r.Use(rateLimiter.Middleware)

		r.Route("/sources", func(r chi.Router) {
			r.Get("/", sourceH.List)
			r.Get("/opml", sourceH.ExportOPML)
			r.Post("/opml/import", sourceH.ImportOPML)
			r.Post("/inoreader/import", sourceH.ImportInoreader)
			r.Get("/stats", sourceH.ItemStats)
			r.Get("/daily-stats", sourceH.DailyStats)
			r.Get("/health", sourceH.Health)
			r.Get("/optimization", sourceH.Optimization)
			r.Get("/navigator", sourceH.Navigator)
			r.Get("/recommended", sourceH.Recommended)
			r.Post("/", sourceH.Create)
			r.Post("/discover", sourceH.Discover)
			r.Get("/suggestions", sourceH.Suggest)
			r.Patch("/{id}", sourceH.Update)
			r.Delete("/{id}", sourceH.Delete)
		})

		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemH.List)
			r.Get("/search-suggestions", itemH.SearchSuggestions)
			r.Get("/favorites/export-markdown", itemH.ExportFavoritesMarkdown)
			r.Get("/stats", itemH.Stats)
			r.Get("/ux-metrics", itemH.UXMetrics)
			r.Get("/topic-trends", itemH.TopicTrends)
			r.Post("/retry-failed", itemH.RetryFailed)
			r.Post("/retry-from-facts-bulk", itemH.RetryFromFactsBulk)
			r.Get("/reading-plan", itemH.ReadingPlan)
			r.Get("/focus-queue", itemH.FocusQueue)
			r.Get("/triage-queue", itemH.TriageQueue)
			r.Get("/today-queue", itemH.TodayQueue)
			r.Get("/triage-all", itemH.TriageAll)
			r.Get("/{id}/related", itemH.Related)
			r.Get("/{id}/navigator", itemH.Navigator)
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
			r.Post("/{id}/restore", itemH.Restore)
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
		r.Post("/ask/navigator", askH.Navigator)
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
		r.Route("/provider-model-snapshots", func(r chi.Router) {
			r.Get("/", providerModelUpdateH.ListSnapshots)
			r.Post("/sync", providerModelUpdateH.SyncSnapshots)
		})
		r.Route("/openrouter-models", func(r chi.Router) {
			r.Get("/", openRouterModelsH.List)
			r.Get("/status", openRouterModelsH.Status)
			r.Post("/sync", openRouterModelsH.Sync)
			r.Put("/overrides/structured-output", openRouterModelsH.UpdateStructuredOutputOverride)
		})
		r.Route("/poe-models", func(r chi.Router) {
			r.Get("/", poeModelsH.List)
			r.Get("/usage", poeModelsH.Usage)
			r.Post("/usage/sync", poeModelsH.SyncUsage)
			r.Get("/status", poeModelsH.Status)
			r.Post("/sync", poeModelsH.Sync)
		})
		r.Route("/aivis-models", func(r chi.Router) {
			r.Get("/", aivisModelsH.List)
			r.Get("/status", aivisModelsH.Status)
			r.Post("/sync", aivisModelsH.Sync)
		})

		r.Get("/briefing/today", briefingH.Today)
		r.Get("/briefing/navigator", briefingH.Navigator)
		r.Route("/ai-navigator-briefs", func(r chi.Router) {
			r.Get("/", aiNavigatorBriefH.List)
			r.Post("/generate", aiNavigatorBriefH.Generate)
			r.Delete("/{id}", aiNavigatorBriefH.Delete)
			r.Get("/{id}", aiNavigatorBriefH.Get)
			r.Post("/{id}/summary-audio-queue", aiNavigatorBriefH.AppendToSummaryAudioQueue)
		})
		r.Route("/audio-briefings", func(r chi.Router) {
			r.Get("/", audioBriefingsH.List)
			r.Post("/generate", audioBriefingsH.Generate)
			r.Post("/{id}/resume", audioBriefingsH.Resume)
			r.Post("/{id}/archive", audioBriefingsH.Archive)
			r.Post("/{id}/unarchive", audioBriefingsH.Unarchive)
			r.Post("/{id}/start-voicing", audioBriefingsH.StartVoicing)
			r.Post("/{id}/start-concat", audioBriefingsH.StartConcat)
			r.Delete("/{id}", audioBriefingsH.Delete)
			r.Get("/{id}", audioBriefingsH.Get)
		})
		r.Post("/summary-audio/items/{id}/synthesize", summaryAudioPlayerH.Synthesize)
		r.Route("/playback-sessions", func(r chi.Router) {
			r.Get("/latest", playbackSessionsH.Latest)
			r.Get("/", playbackSessionsH.List)
			r.Post("/", playbackSessionsH.Create)
			r.Patch("/{id}", playbackSessionsH.Update)
			r.Post("/{id}/complete", playbackSessionsH.Complete)
			r.Post("/{id}/interrupt", playbackSessionsH.Interrupt)
		})
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
			r.Get("/navigator-personas", settingsH.GetNavigatorPersonas)
			r.Get("/preference-profile", settingsH.GetPreferenceProfile)
			r.Get("/preference-profile/summary", settingsH.GetPreferenceProfileSummary)
			r.Delete("/preference-profile", settingsH.ResetPreferenceProfile)
			r.Get("/reading-goals", readingGoalsH.List)
			r.Post("/reading-goals", readingGoalsH.Create)
			r.Patch("/reading-goals/{id}", readingGoalsH.Update)
			r.Post("/reading-goals/{id}/archive", readingGoalsH.Archive)
			r.Post("/reading-goals/{id}/restore", readingGoalsH.Restore)
			r.Delete("/reading-goals/{id}", readingGoalsH.Delete)
			r.Get("/llm-catalog", settingsH.GetLLMCatalog)
			r.Patch("/", settingsH.UpdateBudget)
			r.Patch("/audio-briefing", settingsH.UpdateAudioBriefing)
			r.Patch("/podcast", settingsH.UpdatePodcast)
			r.Post("/podcast-artwork", settingsH.UploadPodcastArtwork)
			r.Patch("/audio-briefing/persona-voices", settingsH.UpdateAudioBriefingPersonaVoices)
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
			r.Post("/moonshot-key", settingsH.SetMoonshotAPIKey)
			r.Delete("/moonshot-key", settingsH.DeleteMoonshotAPIKey)
			r.Post("/xai-key", settingsH.SetXAIAPIKey)
			r.Delete("/xai-key", settingsH.DeleteXAIAPIKey)
			r.Post("/zai-key", settingsH.SetZAIAPIKey)
			r.Delete("/zai-key", settingsH.DeleteZAIAPIKey)
			r.Post("/fireworks-key", settingsH.SetFireworksAPIKey)
			r.Delete("/fireworks-key", settingsH.DeleteFireworksAPIKey)
			r.Post("/poe-key", settingsH.SetPoeAPIKey)
			r.Delete("/poe-key", settingsH.DeletePoeAPIKey)
			r.Post("/siliconflow-key", settingsH.SetSiliconFlowAPIKey)
			r.Delete("/siliconflow-key", settingsH.DeleteSiliconFlowAPIKey)
			r.Post("/openrouter-key", settingsH.SetOpenRouterAPIKey)
			r.Delete("/openrouter-key", settingsH.DeleteOpenRouterAPIKey)
			r.Post("/aivis-key", settingsH.SetAivisAPIKey)
			r.Delete("/aivis-key", settingsH.DeleteAivisAPIKey)
			r.Get("/aivis-user-dictionaries", settingsH.GetAivisUserDictionaries)
			r.Post("/aivis-user-dictionary", settingsH.SetAivisUserDictionary)
			r.Delete("/aivis-user-dictionary", settingsH.DeleteAivisUserDictionary)
			r.Get("/prompt-admin/capabilities", promptAdminH.GetCapabilities)
			r.Get("/prompt-admin/templates", promptAdminH.ListTemplates)
			r.Get("/prompt-admin/templates/{id}", promptAdminH.GetTemplateDetail)
			r.Post("/prompt-admin/templates/{id}/versions", promptAdminH.CreateVersion)
			r.Post("/prompt-admin/templates/{id}/activate", promptAdminH.ActivateTemplateVersion)
			r.Post("/prompt-admin/experiments", promptAdminH.CreateExperiment)
			r.Patch("/prompt-admin/experiments/{id}", promptAdminH.UpdateExperiment)
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

func ensureInngestPutNoContent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w}
		defer func() {
			if r.Method == http.MethodPut && !lrw.wroteHeader {
				if lrw.Header().Get("X-Inngest-Sync-Kind") != "" {
					lrw.WriteHeader(http.StatusNoContent)
				}
			}
		}()

		next.ServeHTTP(lrw, r)
	})
}
