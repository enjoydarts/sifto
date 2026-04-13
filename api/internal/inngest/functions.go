package inngest

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/inngest/inngest/pkg/enums"
	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
)

func generateAudioBriefingsFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	userRepo := repository.NewUserRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	promptTemplateRepo := repository.NewPromptTemplateRepo(db)
	promptResolver := service.NewPromptResolver(promptTemplateRepo)
	secretCipher := service.NewSecretCipher()
	audioConcatRunner := service.NewAudioConcatRunnerFromEnv()
	ttsMarkupPreprocessSvc := service.NewTTSMarkupPreprocessService(userSettingsRepo, secretCipher, worker, llmUsageRepo, cache)
	audioBriefingVoiceRunner := service.NewAudioBriefingVoiceRunner(audioBriefingRepo, userRepo, userSettingsRepo, secretCipher, worker, ttsMarkupPreprocessSvc)
	audioBriefingConcatStarter := service.NewAudioBriefingConcatStarter(audioBriefingRepo, audioConcatRunner)
	orchestrator := service.NewAudioBriefingOrchestrator(audioBriefingRepo, userSettingsRepo, llmUsageRepo, promptResolver, secretCipher, worker, cache, audioBriefingVoiceRunner, audioBriefingConcatStarter)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-audio-briefings", Name: "Generate Audio Briefings"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			settings, err := audioBriefingRepo.ListEnabledSettings(ctx)
			if err != nil {
				return nil, err
			}

			now := timeutil.NowJST()
			processed := 0
			started := 0
			skipped := 0
			failed := 0

			for _, row := range settings {
				processed++
				job, err := orchestrator.GenerateScheduled(ctx, row.UserID, now)
				if err != nil {
					failed++
					log.Printf("generate audio briefing user=%s: %v", row.UserID, err)
					continue
				}
				if job == nil {
					skipped++
					continue
				}
				switch {
				case audioBriefingShouldDispatch(job):
					if _, err := client.Send(ctx, service.NewAudioBriefingRunEvent(row.UserID, job.ID, "scheduled")); err != nil {
						failed++
						log.Printf("enqueue audio briefing run user=%s job=%s: %v", row.UserID, job.ID, err)
						continue
					}
					started++
				case strings.TrimSpace(job.Status) == "skipped":
					skipped++
				default:
					skipped++
				}
			}

			return map[string]any{
				"processed": processed,
				"started":   started,
				"skipped":   skipped,
				"failed":    failed,
			}, nil
		},
	)
}

func generateAINavigatorBriefsFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	briefRepo := repository.NewAINavigatorBriefRepo(db)
	itemRepo := repository.NewItemRepo(db)
	settingsRepo := repository.NewUserSettingsRepo(db)
	userRepo := repository.NewUserRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	secretCipher := service.NewSecretCipher()
	briefService := service.NewAINavigatorBriefService(briefRepo, itemRepo, settingsRepo, userRepo, pushLogRepo, llmUsageRepo, worker, secretCipher, oneSignal, nil, llmUsageCache, nil)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-ai-navigator-briefs", Name: "Generate AI Navigator Briefs"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			now := timeutil.NowJST()
			var slot string
			switch now.Hour() {
			case 8:
				slot = model.AINavigatorBriefSlotMorning
			case 12:
				slot = model.AINavigatorBriefSlotNoon
			case 18:
				slot = model.AINavigatorBriefSlotEvening
			default:
				return map[string]any{
					"status": "skipped",
					"reason": "outside_scheduled_slots",
					"hour":   now.Hour(),
				}, nil
			}

			windowStart, _, err := service.ResolveAINavigatorBriefSlotWindow(now, slot)
			if err != nil {
				return nil, err
			}
			userIDs, err := settingsRepo.ListUserIDsWithAINavigatorBriefEnabled(ctx)
			if err != nil {
				return nil, err
			}

			processed := 0
			enqueued := 0
			skipped := 0
			failed := 0

			for _, userID := range userIDs {
				processed++
				latest, err := briefRepo.LatestBriefByUserSlot(ctx, userID, slot)
				switch {
				case err == nil && latest != nil:
					if !latest.CreatedAt.Before(windowStart) {
						skipped++
						continue
					}
				case err != nil && err != repository.ErrNotFound:
					failed++
					log.Printf("ai navigator brief latest user=%s slot=%s: %v", userID, slot, err)
					continue
				}

				brief, err := briefService.EnqueueBriefForSlot(ctx, userID, slot)
				if err != nil {
					failed++
					log.Printf("ai navigator brief enqueue user=%s slot=%s: %v", userID, slot, err)
					continue
				}
				if brief == nil {
					skipped++
					continue
				}
				if _, err := client.Send(ctx, service.NewAINavigatorBriefRunEvent(userID, brief.ID, "scheduled")); err != nil {
					failed++
					_ = briefRepo.MarkBriefFailedAt(ctx, brief.ID, "failed to enqueue generation", timeutil.NowJST())
					log.Printf("ai navigator brief send run event user=%s brief=%s: %v", userID, brief.ID, err)
					continue
				}
				enqueued++
			}

			return map[string]any{
				"slot":      slot,
				"processed": processed,
				"enqueued":  enqueued,
				"skipped":   skipped,
				"failed":    failed,
			}, nil
		},
	)
}

type aiNavigatorBriefRunEventData struct {
	UserID  string `json:"user_id"`
	BriefID string `json:"brief_id"`
	Trigger string `json:"trigger"`
}

func runAINavigatorBriefPipelineFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, oneSignal *service.OneSignalClient, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	briefRepo := repository.NewAINavigatorBriefRepo(db)
	itemRepo := repository.NewItemRepo(db)
	settingsRepo := repository.NewUserSettingsRepo(db)
	userRepo := repository.NewUserRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	secretCipher := service.NewSecretCipher()
	briefService := service.NewAINavigatorBriefService(briefRepo, itemRepo, settingsRepo, userRepo, pushLogRepo, llmUsageRepo, worker, secretCipher, oneSignal, nil, cache, nil)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:   "run-ai-navigator-brief-pipeline",
			Name: "Run AI Navigator Brief Pipeline",
			Concurrency: []inngestgo.ConfigStepConcurrency{
				{
					Limit: 1,
					Key:   inngestgo.StrPtr("event.data.brief_id"),
					Scope: enums.ConcurrencyScopeFn,
				},
			},
		},
		inngestgo.EventTrigger("ai-navigator-brief/run", nil),
		func(ctx context.Context, input inngestgo.Input[aiNavigatorBriefRunEventData]) (any, error) {
			data := input.Event.Data
			brief, err := briefService.RunQueuedBrief(ctx, data.UserID, data.BriefID)
			if err != nil {
				return nil, err
			}
			if brief == nil {
				return map[string]any{"status": "missing"}, nil
			}
			if brief.Status == model.AINavigatorBriefStatusGenerated {
				if err := briefService.NotifyBrief(ctx, brief); err != nil {
					return nil, err
				}
			}
			return map[string]any{
				"brief_id": data.BriefID,
				"status":   brief.Status,
				"trigger":  data.Trigger,
			}, nil
		},
	)
}

type audioBriefingRunEventData struct {
	UserID  string `json:"user_id"`
	JobID   string `json:"job_id"`
	Trigger string `json:"trigger"`
}

func runAudioBriefingPipelineFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	userRepo := repository.NewUserRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	promptTemplateRepo := repository.NewPromptTemplateRepo(db)
	promptResolver := service.NewPromptResolver(promptTemplateRepo)
	secretCipher := service.NewSecretCipher()
	audioConcatRunner := service.NewAudioConcatRunnerFromEnv()
	ttsMarkupPreprocessSvc := service.NewTTSMarkupPreprocessService(userSettingsRepo, secretCipher, worker, llmUsageRepo, cache)
	audioBriefingVoiceRunner := service.NewAudioBriefingVoiceRunner(audioBriefingRepo, userRepo, userSettingsRepo, secretCipher, worker, ttsMarkupPreprocessSvc)
	audioBriefingConcatStarter := service.NewAudioBriefingConcatStarter(audioBriefingRepo, audioConcatRunner)
	orchestrator := service.NewAudioBriefingOrchestrator(audioBriefingRepo, userSettingsRepo, llmUsageRepo, promptResolver, secretCipher, worker, cache, audioBriefingVoiceRunner, audioBriefingConcatStarter)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:   "run-audio-briefing-pipeline",
			Name: "Run Audio Briefing Pipeline",
			Concurrency: []inngestgo.ConfigStepConcurrency{
				{
					Limit: 1,
					Key:   inngestgo.StrPtr("event.data.job_id"),
					Scope: enums.ConcurrencyScopeFn,
				},
			},
		},
		inngestgo.EventTrigger("audio-briefing/run", nil),
		func(ctx context.Context, input inngestgo.Input[audioBriefingRunEventData]) (any, error) {
			data := input.Event.Data
			if strings.TrimSpace(data.UserID) == "" || strings.TrimSpace(data.JobID) == "" {
				return nil, fmt.Errorf("audio briefing run requires user_id and job_id")
			}
			job, shouldRequeue, err := orchestrator.RunPipelineStep(ctx, strings.TrimSpace(data.UserID), strings.TrimSpace(data.JobID))
			if err != nil {
				return nil, err
			}
			if job == nil {
				return map[string]any{"status": "missing"}, nil
			}
			if shouldRequeue && strings.TrimSpace(job.Status) == "voicing" {
				if _, err := client.Send(ctx, service.NewAudioBriefingRunEvent(job.UserID, job.ID, "continue-voicing")); err != nil {
					return nil, err
				}
			}
			return map[string]any{
				"job_id":         job.ID,
				"user_id":        job.UserID,
				"status":         job.Status,
				"trigger":        strings.TrimSpace(data.Trigger),
				"should_requeue": shouldRequeue,
			}, nil
		},
	)
}

func moveAudioBriefingsToIAFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	archiveSvc := service.NewAudioBriefingArchiveService(audioBriefingRepo, worker)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "move-audio-briefings-to-ia", Name: "Move Audio Briefings To IA"},
		inngestgo.CronTrigger("17 3 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			result, err := archiveSvc.MovePublishedToIA(ctx)
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &service.AudioBriefingArchiveResult{}
			}
			return map[string]any{
				"processed": result.Processed,
				"moved":     result.Moved,
				"failed":    result.Failed,
			}, nil
		},
	)
}

func failStaleAudioBriefingVoicingFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	audioBriefingRepo := repository.NewAudioBriefingRepo(db)
	eventPublisher, err := service.NewEventPublisher()
	if err != nil {
		return nil, err
	}
	staleVoicingSvc := service.NewAudioBriefingStaleVoicingService(audioBriefingRepo).WithPublisher(eventPublisher)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "fail-stale-audio-briefing-voicing", Name: "Fail Stale Audio Briefing Voicing"},
		inngestgo.CronTrigger("*/5 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			result, err := staleVoicingSvc.FailStaleJobs(ctx)
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &service.AudioBriefingStaleVoicingResult{}
			}
			return map[string]any{
				"processed": result.Processed,
				"failed":    result.Failed,
			}, nil
		},
	)
}

func audioBriefingShouldDispatch(job *model.AudioBriefingJob) bool {
	if job == nil {
		return false
	}
	switch strings.TrimSpace(job.Status) {
	case "pending", "scripted", "voiced", "failed":
		return true
	default:
		return false
	}
}

func envFloat64OrDefault(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func envIntOrDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func generateBriefingSnapshotsFn(client inngestgo.Client, db *pgxpool.Pool, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	itemRepo := repository.NewItemRepo(db)
	streakRepo := repository.NewReadingStreakRepo(db)
	snapshotRepo := repository.NewBriefingSnapshotRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	notificationRepo := repository.NewNotificationPriorityRepo(db)
	reviewRepo := repository.NewReviewQueueRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-briefing-snapshots", Name: "Generate Briefing Snapshots"},
		inngestgo.CronTrigger("*/30 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users: %w", err)
			}
			today := timeutil.StartOfDayJST(timeutil.NowJST())
			dateStr := today.Format("2006-01-02")
			updated := 0
			failed := 0
			for _, u := range users {
				payload, err := service.BuildBriefingToday(ctx, itemRepo, streakRepo, u.ID, today, 18)
				if err != nil {
					failed++
					log.Printf("generate-briefing-snapshots build user=%s: %v", u.ID, err)
					continue
				}
				payload.Status = "ready"
				if err := snapshotRepo.Upsert(ctx, u.ID, dateStr, "ready", payload); err != nil {
					failed++
					log.Printf("generate-briefing-snapshots upsert user=%s: %v", u.ID, err)
					continue
				}
				if oneSignal != nil && oneSignal.Enabled() && (len(payload.HighlightItems) > 0 || len(payload.Clusters) > 0) {
					rule, _ := notificationRepo.EnsureDefaults(ctx, u.ID)
					if rule != nil && !rule.BriefingEnabled {
						continue
					}
					alreadyNotified, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "briefing_ready", today)
					if err != nil {
						log.Printf("generate-briefing-snapshots push count user=%s: %v", u.ID, err)
					} else if alreadyNotified == 0 {
						dueReviews, _ := reviewRepo.CountDue(ctx, u.ID, timeutil.NowJST())
						title := "Sifto: 今日のブリーフィングを更新しました"
						message := fmt.Sprintf("注目%d件・クラスタ%d件・再訪%d件を確認できます。", len(payload.HighlightItems), len(payload.Clusters), dueReviews)
						pushRes, pErr := oneSignal.SendToExternalID(
							ctx,
							u.Email,
							title,
							message,
							appPageURL("/"),
							map[string]any{
								"type":         "briefing_ready",
								"briefing_url": appPageURL("/"),
								"date":         dateStr,
								"highlights":   len(payload.HighlightItems),
								"clusters":     len(payload.Clusters),
								"reviews":      dueReviews,
							},
						)
						if pErr != nil {
							log.Printf("generate-briefing-snapshots push send user=%s: %v", u.ID, pErr)
						} else {
							var oneSignalID *string
							recipients := 0
							if pushRes != nil {
								if strings.TrimSpace(pushRes.ID) != "" {
									id := strings.TrimSpace(pushRes.ID)
									oneSignalID = &id
								}
								recipients = pushRes.Recipients
							}
							if err := pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
								UserID:                  u.ID,
								Kind:                    "briefing_ready",
								ItemID:                  nil,
								DayJST:                  today,
								Title:                   title,
								Message:                 message,
								OneSignalNotificationID: oneSignalID,
								Recipients:              recipients,
							}); err != nil {
								log.Printf("generate-briefing-snapshots push log user=%s: %v", u.ID, err)
							}
						}
					}
				}
				updated++
			}
			return map[string]any{
				"date":    dateStr,
				"users":   len(users),
				"updated": updated,
				"failed":  failed,
			}, nil
		},
	)
}

func notifyReviewQueueFn(client inngestgo.Client, db *pgxpool.Pool, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	reviewRepo := repository.NewReviewQueueRepo(db)
	pushLogRepo := repository.NewPushNotificationLogRepo(db)
	notificationRepo := repository.NewNotificationPriorityRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "notify-review-queue", Name: "Notify Review Queue"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if oneSignal == nil || !oneSignal.Enabled() {
				return map[string]any{"enabled": false}, nil
			}
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, err
			}
			now := timeutil.NowJST()
			day := timeutil.StartOfDayJST(now)
			sent := 0
			for _, u := range users {
				rule, _ := notificationRepo.EnsureDefaults(ctx, u.ID)
				if rule != nil && !rule.ReviewEnabled {
					continue
				}
				count, err := reviewRepo.CountDue(ctx, u.ID, now)
				if err != nil || count == 0 {
					continue
				}
				already, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "review_due", day)
				if err != nil || already > 0 {
					continue
				}
				title := "Sifto: 再訪キューがたまっています"
				message := fmt.Sprintf("今日見返したい記事が%d件あります。5分で確認できます。", count)
				pushRes, err := oneSignal.SendToExternalID(ctx, u.Email, title, message, appPageURL("/"), map[string]any{
					"type":       "review_due",
					"target_url": appPageURL("/"),
					"count":      count,
				})
				if err != nil {
					log.Printf("notify-review-queue push user=%s: %v", u.ID, err)
					continue
				}
				var oneSignalID *string
				recipients := 0
				if pushRes != nil {
					if strings.TrimSpace(pushRes.ID) != "" {
						id := strings.TrimSpace(pushRes.ID)
						oneSignalID = &id
					}
					recipients = pushRes.Recipients
				}
				if err := pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
					UserID:                  u.ID,
					Kind:                    "review_due",
					ItemID:                  nil,
					DayJST:                  day,
					Title:                   title,
					Message:                 message,
					OneSignalNotificationID: oneSignalID,
					Recipients:              recipients,
				}); err != nil {
					log.Printf("notify-review-queue push log user=%s: %v", u.ID, err)
				}
				sent++
			}
			return map[string]any{"users": len(users), "sent": sent}, nil
		},
	)
}

func exportObsidianFavoritesFn(client inngestgo.Client, db *pgxpool.Pool, obsidianExport *service.ObsidianExportService) (inngestgo.ServableFunction, error) {
	obsidianRepo := repository.NewObsidianExportRepo(db)
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "export-obsidian-favorites", Name: "Export Obsidian Favorites"},
		inngestgo.CronTrigger("0 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if obsidianExport == nil {
				return map[string]any{"enabled": false}, nil
			}
			configs, err := obsidianRepo.ListEnabled(ctx)
			if err != nil {
				return nil, fmt.Errorf("list enabled obsidian exports: %w", err)
			}
			updated := 0
			skipped := 0
			failed := 0
			for _, cfg := range configs {
				res, runErr := obsidianExport.RunUser(ctx, cfg, 100)
				if runErr != nil {
					failed++
					log.Printf("export-obsidian-favorites user=%s: %v", cfg.UserID, runErr)
					_ = obsidianRepo.MarkRun(ctx, cfg.UserID, false)
					continue
				}
				updated += res.Updated
				skipped += res.Skipped
				failed += res.Failed
			}
			return map[string]any{
				"users":   len(configs),
				"updated": updated,
				"skipped": skipped,
				"failed":  failed,
			}, nil
		},
	)
}

func trackProviderModelUpdatesFn(client inngestgo.Client, db *pgxpool.Pool, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	syncSvc := service.NewProviderModelSnapshotSyncService(
		repository.NewUserRepo(db),
		repository.NewUserSettingsRepo(db),
		updateRepo,
		repository.NewPushNotificationLogRepo(db),
		oneSignal,
		service.NewSecretCipher(),
	)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "track-provider-model-updates", Name: "Track Provider Model Updates"},
		inngestgo.CronTrigger("0 */6 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			result, err := syncSvc.SyncCommonProviders(ctx, "cron")
			if err != nil {
				return nil, err
			}
			return map[string]any{"providers": result.Providers, "changes": result.Changes}, nil
		},
	)
}

func syncOpenRouterModelsFn(client inngestgo.Client, db *pgxpool.Pool, resend *service.ResendClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	modelRepo := repository.NewOpenRouterModelRepo(db)
	updateRepo := repository.NewProviderModelUpdateRepo(db)
	openrouterSvc := service.NewOpenRouterCatalogService()
	openAI := service.NewOpenAIClient()
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "sync-openrouter-models", Name: "Sync OpenRouter Models"},
		inngestgo.CronTrigger("0 3 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			syncRunID, err := modelRepo.StartSyncRun(ctx, "cron")
			if err != nil {
				return nil, err
			}
			fetchedAt := time.Now().UTC()
			models, fetchErr := openrouterSvc.FetchTextGenerationModels(ctx)
			if fetchErr != nil {
				msg := fetchErr.Error()
				_ = modelRepo.FinishSyncRun(ctx, syncRunID, 0, 0, &msg)
				return nil, fetchErr
			}
			models = service.EnrichOpenRouterDescriptionsJA(ctx, modelRepo, openAI, models)
			if err := modelRepo.InsertSnapshots(ctx, syncRunID, fetchedAt, models); err != nil {
				msg := err.Error()
				_ = modelRepo.FinishSyncRun(ctx, syncRunID, 0, 0, &msg)
				return nil, err
			}
			total := 0
			for _, item := range models {
				if item.DescriptionEN != nil && strings.TrimSpace(*item.DescriptionEN) != "" {
					total++
				}
			}
			if err := modelRepo.UpdateTranslationProgress(ctx, syncRunID, total, total); err != nil {
				return nil, err
			}
			if err := modelRepo.FinishSyncRun(ctx, syncRunID, len(models), len(models), nil); err != nil {
				return nil, err
			}
			service.SetDynamicChatModels(service.OpenRouterSnapshotsToCatalogModels(models))

			prevModels, err := modelRepo.ListPreviousSuccessfulSnapshots(ctx, syncRunID)
			if err != nil {
				return nil, err
			}
			addedModelIDs, constrainedModelIDs, removedModelIDs := diffOpenRouterModelAvailability(prevModels, models)
			if len(addedModelIDs) == 0 && len(constrainedModelIDs) == 0 && len(removedModelIDs) == 0 {
				return map[string]any{"fetched": len(models), "added_models": 0, "constrained_models": 0, "removed_models": 0}, nil
			}

			nowJST := timeutil.NowJST()
			changeEvents := make([]model.ProviderModelChangeEvent, 0, len(addedModelIDs)+len(constrainedModelIDs)+len(removedModelIDs))
			for _, modelID := range addedModelIDs {
				changeEvents = append(changeEvents, model.ProviderModelChangeEvent{
					Provider:   "openrouter",
					ChangeType: "added",
					ModelID:    modelID,
					DetectedAt: nowJST,
					Metadata:   map[string]any{"source": "openrouter_sync", "trigger": "cron"},
				})
			}
			for _, modelID := range constrainedModelIDs {
				changeEvents = append(changeEvents, model.ProviderModelChangeEvent{
					Provider:   "openrouter",
					ChangeType: "constrained",
					ModelID:    modelID,
					DetectedAt: nowJST,
					Metadata:   map[string]any{"source": "openrouter_sync", "trigger": "cron"},
				})
			}
			for _, modelID := range removedModelIDs {
				changeEvents = append(changeEvents, model.ProviderModelChangeEvent{
					Provider:   "openrouter",
					ChangeType: "removed",
					ModelID:    modelID,
					DetectedAt: nowJST,
					Metadata:   map[string]any{"source": "openrouter_sync", "trigger": "cron"},
				})
			}
			if len(changeEvents) > 0 {
				if err := updateRepo.InsertChangeEvents(ctx, changeEvents); err != nil {
					return nil, err
				}
			}

			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, err
			}
			title := buildOpenRouterModelAlertTitle(addedModelIDs, constrainedModelIDs, removedModelIDs)
			message := buildOpenRouterModelMessage(addedModelIDs, constrainedModelIDs, removedModelIDs)
			targetURL := appPageURL("/openrouter-models")
			day := timeutil.StartOfDayJST(nowJST)
			pushLogRepo := repository.NewPushNotificationLogRepo(db)
			for _, u := range users {
				alreadyNotified, err := pushLogRepo.CountByUserKindDay(ctx, u.ID, "openrouter_model_update", day)
				if err != nil || alreadyNotified > 0 {
					continue
				}
				var oneSignalID *string
				recipients := 0
				notified := false
				if oneSignal != nil && oneSignal.Enabled() {
					pushRes, pErr := oneSignal.SendToExternalID(
						ctx,
						u.Email,
						title,
						message,
						targetURL,
						map[string]any{
							"type":              "openrouter_model_update",
							"url":               targetURL,
							"added_count":       len(addedModelIDs),
							"constrained_count": len(constrainedModelIDs),
							"removed_count":     len(removedModelIDs),
						},
					)
					if pErr != nil {
						log.Printf("sync-openrouter-models push user=%s: %v", u.ID, pErr)
					} else {
						notified = true
						if pushRes != nil {
							if strings.TrimSpace(pushRes.ID) != "" {
								id := strings.TrimSpace(pushRes.ID)
								oneSignalID = &id
							}
							recipients = pushRes.Recipients
						}
					}
				}
				if resend != nil && resend.Enabled() && strings.TrimSpace(u.Email) != "" {
					if err := resend.SendOpenRouterModelAlert(ctx, u.Email, service.OpenRouterModelAlertEmail{
						Added:       limitStrings(addedModelIDs, 12),
						Constrained: limitStrings(constrainedModelIDs, 12),
						Removed:     limitStrings(removedModelIDs, 12),
						TargetURL:   targetURL,
					}); err != nil {
						log.Printf("sync-openrouter-models email user=%s: %v", u.ID, err)
					} else {
						notified = true
					}
				}
				if !notified {
					continue
				}
				if err := pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
					UserID:                  u.ID,
					Kind:                    "openrouter_model_update",
					ItemID:                  nil,
					DayJST:                  day,
					Title:                   title,
					Message:                 message,
					OneSignalNotificationID: oneSignalID,
					Recipients:              recipients,
				}); err != nil {
					log.Printf("sync-openrouter-models notify log user=%s: %v", u.ID, err)
				}
			}
			return map[string]any{
				"fetched":            len(models),
				"added_models":       len(addedModelIDs),
				"constrained_models": len(constrainedModelIDs),
				"removed_models":     len(removedModelIDs),
			}, nil
		},
	)
}

func syncPoeUsageHistoryFn(client inngestgo.Client, db *pgxpool.Pool, keyProvider *service.UserKeyProvider) (inngestgo.ServableFunction, error) {
	settingsRepo := repository.NewUserSettingsRepo(db)
	poeUsageRepo := repository.NewPoeUsageRepo(db)
	poeUsageSvc := service.NewPoeUsageService(poeUsageRepo)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "sync-poe-usage-history", Name: "Sync Poe Usage History"},
		inngestgo.CronTrigger("0 */6 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			userIDs, err := settingsRepo.ListUserIDsWithPoeAPIKey(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users with poe api key: %w", err)
			}
			synced := 0
			failed := 0
			skipped := 0
			for _, userID := range userIDs {
				id := userID
				apiKey, err := loadUserAPIKey(ctx, keyProvider, &id, "poe")
				if err != nil {
					log.Printf("sync-poe-usage-history load key user=%s: %v", userID, err)
					failed++
					continue
				}
				if apiKey == nil || strings.TrimSpace(*apiKey) == "" {
					skipped++
					continue
				}
				if _, err := poeUsageSvc.SyncHistory(ctx, userID, *apiKey, "cron"); err != nil {
					log.Printf("sync-poe-usage-history sync user=%s: %v", userID, err)
					failed++
					continue
				}
				synced++
			}
			return map[string]any{
				"users":   len(userIDs),
				"synced":  synced,
				"failed":  failed,
				"skipped": skipped,
			}, nil
		},
	)
}

func buildOpenRouterModelAlertTitle(added, constrained, removed []string) string {
	total := len(added) + len(constrained) + len(removed)
	return fmt.Sprintf("Sifto: OpenRouter モデル更新 %d 件", total)
}

func buildOpenRouterModelMessage(added, constrained, removed []string) string {
	parts := make([]string, 0, 3)
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("追加 %d件", len(added)))
	}
	if len(constrained) > 0 {
		parts = append(parts, fmt.Sprintf("制約あり %d件", len(constrained)))
	}
	if len(removed) > 0 {
		parts = append(parts, fmt.Sprintf("削除 %d件", len(removed)))
	}
	if len(parts) == 0 {
		return "OpenRouter のモデル更新を検知しました。"
	}
	return strings.Join(parts, " / ")
}

func limitStrings(in []string, limit int) []string {
	if len(in) <= limit {
		return append([]string{}, in...)
	}
	return append([]string{}, in[:limit]...)
}

func diffOpenRouterModelAvailability(previous, current []repository.OpenRouterModelSnapshot) (added, constrained, removed []string) {
	prevMap := make(map[string]service.OpenRouterModelAvailability, len(previous))
	for _, item := range previous {
		state, _ := service.OpenRouterSnapshotAvailability(item)
		prevMap[item.ModelID] = state
	}
	currMap := make(map[string]service.OpenRouterModelAvailability, len(current))
	for _, item := range current {
		state, _ := service.OpenRouterSnapshotAvailability(item)
		currMap[item.ModelID] = state
		if _, existed := prevMap[item.ModelID]; !existed {
			added = append(added, item.ModelID)
			continue
		}
		if prevMap[item.ModelID] == service.OpenRouterModelAvailable && state == service.OpenRouterModelConstrained {
			constrained = append(constrained, item.ModelID)
		}
	}
	for _, item := range previous {
		if _, exists := currMap[item.ModelID]; !exists {
			removed = append(removed, item.ModelID)
		}
	}
	sort.Strings(added)
	sort.Strings(constrained)
	sort.Strings(removed)
	return added, constrained, removed
}

func fetchRSSFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "fetch-rss", Name: "Fetch RSS Feeds"},
		inngestgo.CronTrigger("*/10 * * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			sources, err := sourceRepo.ListEnabled(ctx)
			if err != nil {
				return nil, fmt.Errorf("list sources: %w", err)
			}

			fp := gofeed.NewParser()
			newCount := 0

			for _, src := range sources {
				feed, err := fp.ParseURLWithContext(src.URL, ctx)
				if err != nil {
					log.Printf("fetch rss %s: %v", src.URL, err)
					_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
					reason := fmt.Sprintf("fetch error: %v", err)
					_ = sourceRepo.RefreshHealthSnapshot(ctx, src.ID, &reason)
					continue
				}

				for _, entry := range feed.Items {
					if entry.Link == "" {
						continue
					}
					var title *string
					if entry.Title != "" {
						title = &entry.Title
					}
					itemID, created, err := itemRepo.UpsertFromFeed(ctx, src.ID, entry.Link, title)
					if err != nil {
						log.Printf("upsert item %s: %v", entry.Link, err)
						continue
					}
					if !created {
						continue
					}
					newCount++
					reason := "fetch_rss"
					titleVal := title
					if _, err := client.Send(ctx, service.NewItemCreatedEvent(itemID, src.ID, entry.Link, titleVal, reason)); err != nil {
						log.Printf("send item/created: %v", err)
					}
				}
				_ = sourceRepo.UpdateLastFetchedAt(ctx, src.ID, timeutil.NowJST())
				_ = sourceRepo.RefreshHealthSnapshot(ctx, src.ID, nil)
			}
			return map[string]int{"new_items": newCount}, nil
		},
	)
}

func processItemFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, openAI *service.OpenAIClient, oneSignal *service.OneSignalClient, keyProvider *service.UserKeyProvider, cache service.JSONCache) (inngestgo.ServableFunction, error) {
	deps := processItemDeps{
		itemRepo:           repository.NewItemInngestRepo(db),
		itemViewRepo:       repository.NewItemRepo(db),
		llmUsageRepo:       repository.NewLLMUsageLogRepo(db),
		llmExecutionRepo:   repository.NewLLMExecutionEventRepo(db),
		sourceRepo:         repository.NewSourceRepo(db),
		userSettingsRepo:   repository.NewUserSettingsRepo(db),
		userRepo:           repository.NewUserRepo(db),
		pushLogRepo:        repository.NewPushNotificationLogRepo(db),
		notificationRepo:   repository.NewNotificationPriorityRepo(db),
		readingGoalRepo:    repository.NewReadingGoalRepo(db),
		promptResolver:     service.NewPromptResolver(repository.NewPromptTemplateRepo(db)),
		worker:             worker,
		openAI:             openAI,
		oneSignal:          oneSignal,
		publisher:          mustEventPublisher(),
		keyProvider:        keyProvider,
		cache:              cache,
		pickScoreThreshold: envFloat64OrDefault("ONESIGNAL_PICK_SCORE_THRESHOLD", 0.90),
		pickMaxPerDay:      envIntOrDefault("ONESIGNAL_PICK_MAX_PER_DAY", 2),
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID:   "process-item",
			Name: "Process Item",
			Concurrency: []inngestgo.ConfigStepConcurrency{
				{
					Limit: 5,
				},
			},
			Throttle: &inngestgo.ConfigThrottle{
				Limit:  30,
				Period: time.Minute,
				Burst:  6,
			},
		},
		inngestgo.EventTrigger("item/created", nil),
		func(ctx context.Context, input inngestgo.Input[processItemEventData]) (any, error) {
			data := input.Event.Data
			ctx = withLLMExecutionTrigger(ctx, data.TriggerID, data.Reason)
			itemID := data.ItemID
			url := data.URL
			var userIDPtr *string
			if data.SourceID != "" {
				if uid, err := deps.sourceRepo.GetUserIDBySourceID(ctx, data.SourceID); err == nil {
					userIDPtr = &uid
				} else {
					log.Printf("process-item source owner lookup failed source_id=%s err=%v", data.SourceID, err)
				}
			}
			var userModelSettings *model.UserSettings
			if userIDPtr != nil && *userIDPtr != "" {
				userModelSettings, _ = deps.userSettingsRepo.GetByUserID(ctx, *userIDPtr)
			}
			log.Printf("process-item start item_id=%s url=%s trigger_id=%s reason=%s", itemID, url, strings.TrimSpace(data.TriggerID), strings.TrimSpace(data.Reason))

			var extracted *service.ExtractBodyResponse
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				stepLabel := "extract-body"
				if attempt > 0 {
					stepLabel = fmt.Sprintf("extract-body-%d", attempt+1)
				}
				extracted, err = step.Run(ctx, stepLabel, func(ctx context.Context) (*service.ExtractBodyResponse, error) {
					log.Printf("process-item extract-body start item_id=%s attempt=%d", itemID, attempt+1)
					return deps.worker.ExtractBody(ctx, url)
				})
				if err == nil {
					break
				}
				persistPartialExtractMetadata(ctx, deps.itemRepo, deps.cache, itemID, service.ExtractBodyPartial(err))
				log.Printf("process-item extract-body failed item_id=%s attempt=%d err=%v", itemID, attempt+1, err)
				if !shouldRetryExtractBody(attempt, err) {
					if shouldDeleteOnExtractBodyFailure(err) {
						return nil, markProcessItemDeleted(ctx, deps.itemRepo, deps.cache, itemID, "extract body retried and deleted", err)
					}
					return nil, markProcessItemFailed(ctx, deps.itemRepo, deps.cache, itemID, "extract body retried and failed", err)
				}
			}
			log.Printf("process-item extract-body done item_id=%s content_len=%d", itemID, len(extracted.Content))
			if reason := invalidExtractReason(extracted.Title, extracted.Content); reason != "" {
				log.Printf("process-item invalid-extract deleted item_id=%s reason=%s", itemID, reason)
				return nil, markProcessItemDeleted(ctx, deps.itemRepo, deps.cache, itemID, reason, fmt.Errorf("content rejected after extract"))
			}

			if err := updateItemAfterExtract(ctx, deps.itemRepo, itemID, extracted); err != nil {
				log.Printf("process-item update-after-extract failed item_id=%s err=%v", itemID, err)
				return nil, fmt.Errorf("update after extract: %w", err)
			}
			bumpProcessItemDetailCacheVersion(ctx, deps.cache, itemID)
			log.Printf("process-item update-after-extract done item_id=%s", itemID)
			titleForLLM := resolveProcessItemTitleForLLM(extracted.Title, data.Title)
			factsStage, err := extractAndPersistFacts(ctx, deps, data, itemID, userIDPtr, userModelSettings, titleForLLM, extracted.Content)
			if err != nil {
				return nil, err
			}
			summaryStage, err := summarizeAndPersistItem(ctx, deps, data, itemID, userIDPtr, userModelSettings, titleForLLM, extracted.Content, factsStage.Facts.Facts)
			if err != nil {
				return nil, err
			}
			sendPickNotificationIfNeeded(ctx, deps, itemID, url, userIDPtr, titleForLLM, summaryStage.Summary)
			createEmbeddingIfPossible(ctx, deps, data, itemID, userIDPtr, userModelSettings, titleForLLM, summaryStage.Summary, factsStage.Facts.Facts)
			log.Printf("process-item complete item_id=%s", itemID)

			return map[string]string{"item_id": itemID, "status": "summarized"}, nil
		},
	)
}

func mustEventPublisher() *service.EventPublisher {
	publisher, err := service.NewEventPublisher()
	if err != nil {
		log.Fatalf("event publisher: %v", err)
	}
	return publisher
}

func embedItemFn(client inngestgo.Client, db *pgxpool.Pool, openAI *service.OpenAIClient, keyProvider *service.UserKeyProvider) (inngestgo.ServableFunction, error) {
	itemRepo := repository.NewItemInngestRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	llmExecutionRepo := repository.NewLLMExecutionEventRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	type EventData struct {
		ItemID   string `json:"item_id"`
		SourceID string `json:"source_id"`
	}

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "embed-item", Name: "Create Item Embedding"},
		inngestgo.EventTrigger("item/embed", nil),
		func(ctx context.Context, input inngestgo.Input[EventData]) (any, error) {
			data := input.Event.Data
			if data.ItemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}

			candidate, err := itemRepo.GetEmbeddingCandidate(ctx, data.ItemID)
			if err != nil {
				return nil, fmt.Errorf("get embedding candidate: %w", err)
			}
			userID := candidate.UserID
			userOpenAIKey, err := loadUserAPIKey(ctx, keyProvider, &userID, "openai")
			if err != nil {
				return nil, err
			}
			userModelSettings, _ := userSettingsRepo.GetByUserID(ctx, userID)

			inputText := buildItemEmbeddingInput(candidate.Title, candidate.Summary, candidate.Topics, candidate.Facts)
			embModel := service.OpenAIEmbeddingModel()
			if userModelSettings != nil && userModelSettings.EmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*userModelSettings.EmbeddingModel) {
				embModel = *userModelSettings.EmbeddingModel
			}
			embResp, err := step.Run(ctx, "create-embedding", func(ctx context.Context) (*service.CreateEmbeddingResponse, error) {
				return openAI.CreateEmbedding(ctx, *userOpenAIKey, embModel, inputText)
			})
			if err != nil {
				recordLLMExecutionFailure(ctx, llmExecutionRepo, "embedding", &embModel, 0, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil, nil, err)
				return nil, err
			}
			if err := itemRepo.UpsertEmbedding(ctx, candidate.ItemID, embModel, embResp.Embedding); err != nil {
				return nil, fmt.Errorf("upsert embedding: %w", err)
			}

			recordLLMUsage(ctx, llmUsageRepo, "embedding", embResp.LLM, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil, nil)
			recordLLMExecutionSuccess(ctx, llmExecutionRepo, "embedding", embResp.LLM, 0, &candidate.UserID, &candidate.SourceID, &candidate.ItemID, nil, nil)
			return map[string]any{
				"item_id":    candidate.ItemID,
				"source_id":  candidate.SourceID,
				"dimensions": len(embResp.Embedding),
				"status":     "embedded",
				"model":      embModel,
			}, nil
		},
	)
}

func buildItemEmbeddingInput(title *string, summary string, topics, facts []string) string {
	out := ""
	if title != nil && *title != "" {
		out += "title: " + *title + "\n"
	}
	if summary != "" {
		out += "summary: " + summary + "\n"
	}
	if len(topics) > 0 {
		out += "topics: " + fmt.Sprintf("%v", topics) + "\n"
	}
	if len(facts) > 0 {
		out += "facts:\n"
		limit := len(facts)
		if limit > 12 {
			limit = 12
		}
		for i := 0; i < limit; i++ {
			out += "- " + facts[i] + "\n"
		}
	}
	return out
}

func generateDigestFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	userRepo := repository.NewUserRepo(db)
	itemRepo := repository.NewItemInngestRepo(db)
	digestRepo := repository.NewDigestInngestRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "generate-digest", Name: "Generate Daily Digest"},
		inngestgo.CronTrigger("0 21 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			users, err := userRepo.ListAll(ctx)
			if err != nil {
				return nil, fmt.Errorf("list users: %w", err)
			}

			today := timeutil.StartOfDayJST(timeutil.NowJST())
			since := today.AddDate(0, 0, -1)

			created := 0
			skippedSent := 0
			for _, u := range users {
				items, err := itemRepo.ListSummarizedForUser(ctx, u.ID, since, today)
				if err != nil || len(items) == 0 {
					continue
				}

				digestID, alreadySent, err := digestRepo.Create(ctx, u.ID, today, items)
				if err != nil {
					log.Printf("create digest for %s: %v", u.Email, err)
					continue
				}
				if alreadySent {
					skippedSent++
					continue
				}

				if _, err := client.Send(ctx, inngestgo.Event{
					Name: "digest/created",
					Data: map[string]any{
						"digest_id": digestID,
						"user_id":   u.ID,
						"to":        u.Email,
					},
				}); err != nil {
					log.Printf("send digest/created: %v", err)
				}
				created++
			}
			return map[string]int{
				"digests_created":      created,
				"digests_skipped_sent": skippedSent,
			}, nil
		},
	)
}

func composeDigestCopyFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, keyProvider *service.UserKeyProvider) (inngestgo.ServableFunction, error) {
	digestRepo := repository.NewDigestInngestRepo(db)
	itemRepo := repository.NewItemRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)
	llmExecutionRepo := repository.NewLLMExecutionEventRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)
	promptResolver := service.NewPromptResolver(repository.NewPromptTemplateRepo(db))

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "compose-digest-copy", Name: "Compose Digest Email Copy"},
		inngestgo.EventTrigger("digest/created", nil),
		func(ctx context.Context, input inngestgo.Input[DigestCreatedData]) (any, error) {
			data := input.Event.Data
			log.Printf("compose-digest-copy start digest_id=%s", data.DigestID)
			markStatus := func(status string, sendErr error) {
				var msg *string
				if sendErr != nil {
					s := sendErr.Error()
					if len(s) > 2000 {
						s = s[:2000]
					}
					msg = &s
				}
				if err := digestRepo.UpdateSendStatus(ctx, data.DigestID, status, msg); err != nil {
					log.Printf("compose-digest-copy update-status failed digest_id=%s status=%s err=%v", data.DigestID, status, err)
				}
			}
			userModelSettings, _ := userSettingsRepo.GetByUserID(ctx, data.UserID)

			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				markStatus("fetch_failed", err)
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			log.Printf("compose-digest-copy fetched digest_id=%s items=%d", data.DigestID, len(digest.Items))

			if len(digest.Items) == 0 {
				log.Printf("compose-digest-copy skip-no-items digest_id=%s", data.DigestID)
				markStatus("skipped_no_items", nil)
				return map[string]string{"status": "skipped", "reason": "no items"}, nil
			}
			markStatus("processing", nil)

			if digest.EmailSubject != nil && digest.EmailBody != nil {
				log.Printf("compose-digest-copy reuse-copy digest_id=%s", data.DigestID)
			} else {
				_, err := step.Run(ctx, "compose-digest-copy", func(ctx context.Context) (string, error) {
					if err := composeDigestEmailCopy(ctx, digestRepo, itemRepo, userSettingsRepo, llmUsageRepo, llmExecutionRepo, processItemDeps{worker: worker, keyProvider: keyProvider, promptResolver: promptResolver}, data, digest, userModelSettings); err != nil {
						return "", err
					}
					return "stored", nil
				})
				if err != nil {
					markStatus("compose_failed", err)
					return nil, fmt.Errorf("compose digest copy: %w", err)
				}
			}

			if _, err := client.Send(ctx, inngestgo.Event{
				Name: "digest/copy-composed",
				Data: map[string]any{
					"digest_id": data.DigestID,
					"user_id":   data.UserID,
					"to":        data.To,
				},
			}); err != nil {
				markStatus("enqueue_send_failed", err)
				return nil, fmt.Errorf("send digest/copy-composed: %w", err)
			}
			log.Printf("compose-digest-copy complete digest_id=%s", data.DigestID)
			return map[string]string{"status": "composed", "digest_id": data.DigestID}, nil
		},
	)
}

func sendDigestFn(client inngestgo.Client, db *pgxpool.Pool, worker *service.WorkerClient, resend *service.ResendClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	_ = worker
	digestRepo := repository.NewDigestInngestRepo(db)
	userSettingsRepo := repository.NewUserSettingsRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "send-digest", Name: "Send Digest Email"},
		inngestgo.EventTrigger("digest/copy-composed", nil),
		func(ctx context.Context, input inngestgo.Input[DigestCopyComposedData]) (any, error) {
			data := input.Event.Data
			log.Printf("send-digest start digest_id=%s to=%s", data.DigestID, data.To)
			markStatus := func(status string, sendErr error) {
				var msg *string
				if sendErr != nil {
					s := sendErr.Error()
					if len(s) > 2000 {
						s = s[:2000]
					}
					msg = &s
				}
				if err := digestRepo.UpdateSendStatus(ctx, data.DigestID, status, msg); err != nil {
					log.Printf("send-digest update-status failed digest_id=%s status=%s err=%v", data.DigestID, status, err)
				}
			}

			digest, err := digestRepo.GetForEmail(ctx, data.DigestID)
			if err != nil {
				markStatus("fetch_failed", err)
				return nil, fmt.Errorf("fetch digest: %w", err)
			}
			if digest.EmailSubject == nil || digest.EmailBody == nil {
				err := fmt.Errorf("digest email copy is missing")
				markStatus("compose_failed", err)
				return nil, err
			}
			if !resend.Enabled() {
				markStatus("skipped_resend_disabled", nil)
				return map[string]string{"status": "skipped", "reason": "resend_disabled"}, nil
			}
			digestEmailEnabled, err := userSettingsRepo.IsDigestEmailEnabled(ctx, data.UserID)
			if err != nil {
				markStatus("user_settings_failed", err)
				return nil, fmt.Errorf("load user digest email setting: %w", err)
			}
			if !digestEmailEnabled {
				markStatus("skipped_user_disabled", nil)
				return map[string]string{"status": "skipped", "reason": "user_disabled"}, nil
			}
			markStatus("processing", nil)

			_, err = step.Run(ctx, "send-email", func(ctx context.Context) (string, error) {
				if err := resend.SendDigest(ctx, data.To, digest, &service.DigestEmailCopy{
					Subject: *digest.EmailSubject,
					Body:    *digest.EmailBody,
				}); err != nil {
					return "", err
				}
				return "sent", nil
			})
			if err != nil {
				markStatus("send_email_failed", err)
				return nil, fmt.Errorf("send email: %w", err)
			}
			if err := digestRepo.UpdateSentAt(ctx, data.DigestID); err != nil {
				log.Printf("update sent_at: %v", err)
			}
			if oneSignal != nil && oneSignal.Enabled() {
				_, pErr := oneSignal.SendToExternalID(
					ctx,
					data.To,
					"Sifto: ダイジェストを配信しました",
					fmt.Sprintf("%s のダイジェストを配信しました。", digest.DigestDate),
					appPageURL("/digests/"+data.DigestID),
					map[string]any{
						"type":       "digest_sent",
						"digest_id":  data.DigestID,
						"digest_url": appPageURL("/digests/" + data.DigestID),
					},
				)
				if pErr != nil {
					log.Printf("send-digest push failed digest_id=%s to=%s: %v", data.DigestID, data.To, pErr)
				}
			}
			log.Printf("send-digest complete digest_id=%s", data.DigestID)
			return map[string]string{"status": "sent", "to": data.To}, nil
		},
	)
}

func checkBudgetAlertsFn(client inngestgo.Client, db *pgxpool.Pool, resend *service.ResendClient, oneSignal *service.OneSignalClient) (inngestgo.ServableFunction, error) {
	settingsRepo := repository.NewUserSettingsRepo(db)
	alertLogRepo := repository.NewBudgetAlertLogRepo(db)
	forecastAlertLogRepo := repository.NewBudgetForecastAlertLogRepo(db)
	llmUsageRepo := repository.NewLLMUsageLogRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "check-budget-alerts", Name: "Check Monthly Budget Alerts"},
		inngestgo.CronTrigger("0 0 * * *"),
		func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
			if (resend == nil || !resend.Enabled()) && (oneSignal == nil || !oneSignal.Enabled()) {
				return map[string]any{"status": "skipped", "reason": "no_budget_alert_channel"}, nil
			}

			targets, err := settingsRepo.ListBudgetAlertTargets(ctx)
			if err != nil {
				return nil, fmt.Errorf("list budget alert targets: %w", err)
			}

			nowJST := timeutil.NowJST()
			monthStartJST := time.Date(nowJST.Year(), nowJST.Month(), 1, 0, 0, 0, 0, timeutil.JST)
			nextMonthJST := monthStartJST.AddDate(0, 1, 0)
			daysInMonth := nextMonthJST.AddDate(0, 0, -1).Day()
			elapsedDays := nowJST.Day()
			checked := 0
			sent := 0
			skipped := 0

			for _, tgt := range targets {
				checked++
				usedCostUSD, err := llmUsageRepo.SumEstimatedCostByUserBetween(ctx, tgt.UserID, monthStartJST, nextMonthJST)
				if err != nil {
					log.Printf("check-budget-alerts sum cost user_id=%s: %v", tgt.UserID, err)
					continue
				}
				if tgt.MonthlyBudgetUSD <= 0 {
					skipped++
					continue
				}
				remainingRatio := (tgt.MonthlyBudgetUSD - usedCostUSD) / tgt.MonthlyBudgetUSD
				thresholdRatio := float64(tgt.BudgetAlertThresholdPct) / 100.0
				remainingUSD := tgt.MonthlyBudgetUSD - usedCostUSD
				monthAvgDailyPace := 0.0
				if elapsedDays > 0 {
					monthAvgDailyPace = usedCostUSD / float64(elapsedDays)
				}
				forecastCostUSD := monthAvgDailyPace * float64(daysInMonth)
				forecastDeltaUSD := forecastCostUSD - tgt.MonthlyBudgetUSD

				sentThisTarget := false

				if remainingRatio < thresholdRatio {
					alreadySent, err := alertLogRepo.Exists(ctx, tgt.UserID, monthStartJST, tgt.BudgetAlertThresholdPct)
					if err != nil {
						log.Printf("check-budget-alerts exists user_id=%s: %v", tgt.UserID, err)
					} else if !alreadySent {
						emailSent := false
						pushSent := false
						if resend != nil && resend.Enabled() {
							if err := resend.SendBudgetAlert(ctx, tgt.Email, service.BudgetAlertEmail{
								MonthJST:           monthStartJST.Format("2006-01"),
								MonthlyBudgetUSD:   tgt.MonthlyBudgetUSD,
								UsedCostUSD:        usedCostUSD,
								RemainingBudgetUSD: remainingUSD,
								RemainingPct:       remainingRatio * 100,
								ThresholdPct:       tgt.BudgetAlertThresholdPct,
							}); err != nil {
								log.Printf("check-budget-alerts send user_id=%s email=%s: %v", tgt.UserID, tgt.Email, err)
							} else {
								emailSent = true
							}
						}
						if oneSignal != nil && oneSignal.Enabled() {
							if _, pErr := oneSignal.SendToExternalID(
								ctx,
								tgt.Email,
								"Sifto: 月次LLM予算アラート",
								fmt.Sprintf("残り予算がしきい値(%d%%)を下回りました。", tgt.BudgetAlertThresholdPct),
								appPageURL("/llm-usage"),
								map[string]any{
									"type":          "budget_alert",
									"month_jst":     monthStartJST.Format("2006-01"),
									"threshold_pct": tgt.BudgetAlertThresholdPct,
									"target_url":    appPageURL("/llm-usage"),
								},
							); pErr != nil {
								log.Printf("check-budget-alerts push user_id=%s email=%s: %v", tgt.UserID, tgt.Email, pErr)
							} else {
								pushSent = true
							}
						}
						if emailSent || pushSent {
							if err := alertLogRepo.Insert(ctx, tgt.UserID, monthStartJST, tgt.BudgetAlertThresholdPct, tgt.MonthlyBudgetUSD, usedCostUSD, remainingRatio); err != nil {
								log.Printf("check-budget-alerts log user_id=%s: %v", tgt.UserID, err)
							}
							sentThisTarget = true
						}
					}
				}

				shouldForecastAlert := usedCostUSD > tgt.MonthlyBudgetUSD || (elapsedDays >= 3 && forecastDeltaUSD > 0)
				if shouldForecastAlert {
					forecastSent, err := forecastAlertLogRepo.Exists(ctx, tgt.UserID, monthStartJST)
					if err != nil {
						log.Printf("check-budget-alerts forecast exists user_id=%s: %v", tgt.UserID, err)
					} else if !forecastSent {
						emailSent := false
						pushSent := false
						if resend != nil && resend.Enabled() {
							if err := resend.SendBudgetForecastAlert(ctx, tgt.Email, service.BudgetForecastAlertEmail{
								MonthJST:         monthStartJST.Format("2006-01"),
								MonthlyBudgetUSD: tgt.MonthlyBudgetUSD,
								UsedCostUSD:      usedCostUSD,
								ForecastCostUSD:  forecastCostUSD,
								ForecastDeltaUSD: forecastDeltaUSD,
							}); err != nil {
								log.Printf("check-budget-alerts forecast email user_id=%s email=%s: %v", tgt.UserID, tgt.Email, err)
							} else {
								emailSent = true
							}
						}
						if oneSignal != nil && oneSignal.Enabled() {
							message := fmt.Sprintf("月末着地予測が予算を $%.4f 上回っています。", forecastDeltaUSD)
							if usedCostUSD > tgt.MonthlyBudgetUSD {
								message = "今月のLLM予算をすでに超過しています。"
							}
							if _, pErr := oneSignal.SendToExternalID(
								ctx,
								tgt.Email,
								"Sifto: 月次LLM予算の着地予測アラート",
								message,
								appPageURL("/llm-usage"),
								map[string]any{
									"type":               "budget_forecast_alert",
									"month_jst":          monthStartJST.Format("2006-01"),
									"forecast_cost_usd":  forecastCostUSD,
									"forecast_delta_usd": forecastDeltaUSD,
									"target_url":         appPageURL("/llm-usage"),
								},
							); pErr != nil {
								log.Printf("check-budget-alerts forecast push user_id=%s email=%s: %v", tgt.UserID, tgt.Email, pErr)
							} else {
								pushSent = true
							}
						}
						if emailSent || pushSent {
							if err := forecastAlertLogRepo.Insert(ctx, tgt.UserID, monthStartJST, tgt.MonthlyBudgetUSD, usedCostUSD, forecastCostUSD, forecastDeltaUSD); err != nil {
								log.Printf("check-budget-alerts forecast log user_id=%s: %v", tgt.UserID, err)
							}
							sentThisTarget = true
						}
					}
				}

				if sentThisTarget {
					sent++
				} else {
					skipped++
				}
			}

			return map[string]any{
				"checked":   checked,
				"sent":      sent,
				"skipped":   skipped,
				"month_jst": monthStartJST.Format("2006-01"),
			}, nil
		},
	)
}
