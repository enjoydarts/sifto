package inngest

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
	"github.com/inngest/inngestgo/step"
)

type processItemEventData struct {
	ItemID   string `json:"item_id"`
	SourceID string `json:"source_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
}

type processItemDeps struct {
	itemRepo           *repository.ItemInngestRepo
	itemViewRepo       *repository.ItemRepo
	llmUsageRepo       *repository.LLMUsageLogRepo
	llmExecutionRepo   *repository.LLMExecutionEventRepo
	sourceRepo         *repository.SourceRepo
	userSettingsRepo   *repository.UserSettingsRepo
	userRepo           *repository.UserRepo
	pushLogRepo        *repository.PushNotificationLogRepo
	notificationRepo   *repository.NotificationPriorityRepo
	readingGoalRepo    *repository.ReadingGoalRepo
	worker             *service.WorkerClient
	openAI             *service.OpenAIClient
	oneSignal          *service.OneSignalClient
	secretCipher       *service.SecretCipher
	pickScoreThreshold float64
	pickMaxPerDay      int
}

type processFactsAttemptResult struct {
	Facts   *service.ExtractFactsResponse
	Runtime *llmRuntime
}

type processSummaryAttemptResult struct {
	Summary *service.SummarizeResponse
	Runtime *llmRuntime
}

type processFactsStageResult struct {
	Facts      *service.ExtractFactsResponse
	Check      *service.FactsCheckResponse
	RetryCount int
}

type processSummaryStageResult struct {
	Summary    *service.SummarizeResponse
	Check      *service.SummaryFaithfulnessResponse
	RetryCount int
}

func isTransientLLMWorkerError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	transientHints := []string{
		"status=429",
		"status 429",
		"code\":429",
		"rate limit",
		"status=502",
		"status 502",
		"code\":502",
		"provider returned error",
		"empty choices",
		"context deadline exceeded",
		"timeout",
		"timed out",
		"overload",
		"temporarily unavailable",
	}
	for _, hint := range transientHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}
	return false
}

func canUseLLMFallback(primaryModel, fallbackModel *string, err error) bool {
	if !isTransientLLMWorkerError(err) || fallbackModel == nil {
		return false
	}
	fallback := strings.TrimSpace(*fallbackModel)
	if fallback == "" {
		return false
	}
	if primaryModel != nil && strings.EqualFold(strings.TrimSpace(*primaryModel), fallback) {
		return false
	}
	return true
}

func fallbackFactsCheckWarning(err error) *service.FactsCheckResponse {
	comment := "事実抽出チェックの判定取得に失敗したため要確認です。"
	if err != nil {
		msg := strings.TrimSpace(err.Error())
		if msg != "" {
			comment = fmt.Sprintf("事実抽出チェックの判定取得に失敗したため要確認です: %s", msg)
		}
	}
	if len(comment) > 240 {
		comment = comment[:240]
	}
	return &service.FactsCheckResponse{
		Verdict:      "warn",
		ShortComment: comment,
	}
}

func resolveProcessItemTitleForLLM(extractedTitle *string, fallbackTitle string) *string {
	titleForLLM := extractedTitle
	if titleForLLM == nil || strings.TrimSpace(*titleForLLM) == "" {
		eventTitle := strings.TrimSpace(fallbackTitle)
		if eventTitle != "" {
			titleForLLM = &eventTitle
		}
	}
	return titleForLLM
}

func markProcessItemFailed(ctx context.Context, itemRepo *repository.ItemInngestRepo, itemID, stage string, err error) error {
	msg := fmt.Sprintf("%s: %v", stage, err)
	_ = itemRepo.MarkFailed(ctx, itemID, &msg)
	return fmt.Errorf("%s: %w", stage, err)
}

func extractAndPersistFacts(
	ctx context.Context,
	deps processItemDeps,
	data processItemEventData,
	itemID string,
	userIDPtr *string,
	userModelSettings *model.UserSettings,
	titleForLLM *string,
	content string,
) (*processFactsStageResult, error) {
	const maxFactsCheckRetries = 2

	var factsResp *service.ExtractFactsResponse
	var finalFactsCheck *service.FactsCheckResponse
	var factsRetryCount int

	for attempt := 0; attempt <= maxFactsCheckRetries; attempt++ {
		stepLabel := "extract-facts"
		if attempt > 0 {
			stepLabel = fmt.Sprintf("extract-facts-%d", attempt+1)
		}
		var primaryModelOverride *string
		var fallbackModelOverride *string
		if userModelSettings != nil {
			primaryModelOverride = ptrStringOrNil(userModelSettings.FactsModel)
			fallbackModelOverride = ptrStringOrNil(userModelSettings.FactsFallbackModel)
		}
		factsAttempt, err := step.Run(ctx, stepLabel, func(ctx context.Context) (*processFactsAttemptResult, error) {
			log.Printf("process-item extract-facts start item_id=%s attempt=%d", itemID, attempt+1)
			runtime, err := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, primaryModelOverride, "facts")
			if err != nil {
				return nil, err
			}
			workerCtx := service.WithWorkerTraceMetadata(ctx, "facts", userIDPtr, &data.SourceID, &itemID, nil)
			resp, err := deps.worker.ExtractFactsWithModel(
				workerCtx,
				titleForLLM,
				content,
				runtime.AnthropicKey,
				runtime.GoogleKey,
				runtime.GroqKey,
				runtime.DeepSeekKey,
				runtime.AlibabaKey,
				runtime.MistralKey,
				runtime.XAIKey,
				runtime.ZAIKey,
				runtime.FireworksKey,
				runtime.OpenAIKey,
				runtime.Model,
			)
			if err != nil {
				return nil, err
			}
			return &processFactsAttemptResult{
				Facts:   resp,
				Runtime: runtime,
			}, nil
		})
		if err != nil {
			var failedModel *string
			if factsAttempt != nil && factsAttempt.Runtime != nil {
				failedModel = factsAttempt.Runtime.Model
			}
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "facts", failedModel, attempt, userIDPtr, &data.SourceID, &itemID, nil, err)
			if canUseLLMFallback(primaryModelOverride, fallbackModelOverride, err) {
				fallbackStepLabel := stepLabel + "-fallback"
				fallbackAttempt, fallbackErr := step.Run(ctx, fallbackStepLabel, func(ctx context.Context) (*processFactsAttemptResult, error) {
					log.Printf("process-item extract-facts fallback start item_id=%s attempt=%d", itemID, attempt+1)
					runtime, runtimeErr := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, fallbackModelOverride, "facts")
					if runtimeErr != nil {
						return nil, runtimeErr
					}
					workerCtx := service.WithWorkerTraceMetadata(ctx, "facts", userIDPtr, &data.SourceID, &itemID, nil)
					resp, workerErr := deps.worker.ExtractFactsWithModel(
						workerCtx,
						titleForLLM,
						content,
						runtime.AnthropicKey,
						runtime.GoogleKey,
						runtime.GroqKey,
						runtime.DeepSeekKey,
						runtime.AlibabaKey,
						runtime.MistralKey,
						runtime.XAIKey,
						runtime.ZAIKey,
						runtime.FireworksKey,
						runtime.OpenAIKey,
						runtime.Model,
					)
					if workerErr != nil {
						return nil, workerErr
					}
					return &processFactsAttemptResult{Facts: resp, Runtime: runtime}, nil
				})
				if fallbackErr != nil {
					var fallbackFailedModel *string
					if fallbackAttempt != nil && fallbackAttempt.Runtime != nil {
						fallbackFailedModel = fallbackAttempt.Runtime.Model
					}
					recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "facts", fallbackFailedModel, attempt, userIDPtr, &data.SourceID, &itemID, nil, fallbackErr)
					return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "extract facts", fallbackErr)
				}
				factsAttempt = fallbackAttempt
			} else {
				return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "extract facts", err)
			}
		}

		factsResp = factsAttempt.Facts
		recordLLMUsage(ctx, deps.llmUsageRepo, "facts", factsResp.LLM, userIDPtr, &data.SourceID, &itemID, nil)
		if len(factsResp.Facts) == 0 {
			err := fmt.Errorf("empty facts returned from worker")
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "facts", factsAttempt.Runtime.Model, attempt, userIDPtr, &data.SourceID, &itemID, nil, err)
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "extract facts", err)
		}
		recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, "facts", factsResp.LLM, attempt, userIDPtr, &data.SourceID, &itemID, nil)
		log.Printf("process-item extract-facts done item_id=%s facts=%d attempt=%d", itemID, len(factsResp.Facts), attempt+1)

		var factsCheckModel *string
		if userModelSettings != nil {
			factsCheckModel = ptrStringOrNil(userModelSettings.FactsCheckModel)
		}
		factsCheck, shouldRetry, err := executeLLMCheck(ctx, deps, llmCheckConfig[service.FactsCheckResponse]{
			baseStepName:   "check-facts",
			purpose:        "facts_check",
			resolvePurpose: "facts",
			attempt:        attempt,
			userID:         userIDPtr,
			sourceID:       &data.SourceID,
			itemID:         &itemID,
			modelOverride:  factsCheckModel,
			defaultRuntime: factsAttempt.Runtime,
			call: func(runtime *llmRuntime) (*service.FactsCheckResponse, error) {
				workerCtx := service.WithWorkerTraceMetadata(ctx, "facts_check", userIDPtr, &data.SourceID, &itemID, nil)
				return deps.worker.CheckFactsWithModel(
					workerCtx,
					titleForLLM,
					content,
					factsResp.Facts,
					runtime.AnthropicKey,
					runtime.GoogleKey,
					runtime.GroqKey,
					runtime.DeepSeekKey,
					runtime.AlibabaKey,
					runtime.MistralKey,
					runtime.XAIKey,
					runtime.ZAIKey,
					runtime.FireworksKey,
					runtime.OpenAIKey,
					runtime.Model,
				)
			},
			getLLM:     func(result *service.FactsCheckResponse) *service.LLMUsage { return result.LLM },
			getVerdict: func(result *service.FactsCheckResponse) string { return result.Verdict },
			onExecutionError: func(err error) *service.FactsCheckResponse {
				return fallbackFactsCheckWarning(err)
			},
		})
		if err != nil {
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "facts check", err)
		}

		finalFactsCheck = factsCheck
		if !shouldRetry || attempt >= maxFactsCheckRetries {
			factsRetryCount = attempt
			break
		}
		log.Printf("process-item facts_check retry item_id=%s attempt=%d verdict=%s comment=%s", itemID, attempt+1, factsCheck.Verdict, factsCheck.ShortComment)
	}

	if factsResp == nil {
		return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "extract facts", fmt.Errorf("facts extraction produced no result"))
	}
	if finalFactsCheck == nil {
		return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "facts check", fmt.Errorf("facts check produced no result"))
	}
	if err := deps.itemRepo.InsertFacts(ctx, itemID, factsResp.Facts); err != nil {
		return nil, fmt.Errorf("insert facts: %w", err)
	}
	var factsCheckComment *string
	if comment := strings.TrimSpace(finalFactsCheck.ShortComment); comment != "" {
		factsCheckComment = &comment
	}
	if err := deps.itemRepo.UpsertFactsCheck(ctx, itemID, finalFactsCheck.Verdict, factsRetryCount, factsCheckComment); err != nil {
		return nil, fmt.Errorf("upsert facts check: %w", err)
	}
	if finalFactsCheck.Verdict == "fail" {
		return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "facts check", fmt.Errorf("%s", finalFactsCheck.ShortComment))
	}
	log.Printf("process-item insert-facts done item_id=%s retries=%d facts_check=%s", itemID, factsRetryCount, finalFactsCheck.Verdict)

	return &processFactsStageResult{
		Facts:      factsResp,
		Check:      finalFactsCheck,
		RetryCount: factsRetryCount,
	}, nil
}

func summarizeAndPersistItem(
	ctx context.Context,
	deps processItemDeps,
	data processItemEventData,
	itemID string,
	userIDPtr *string,
	userModelSettings *model.UserSettings,
	titleForLLM *string,
	sourceContent string,
	facts []string,
) (*processSummaryStageResult, error) {
	const maxSummaryFaithfulnessRetries = 2

	var summary *service.SummarizeResponse
	var finalFaithfulness *service.SummaryFaithfulnessResponse
	var summaryRetryCount int

	for attempt := 0; attempt <= maxSummaryFaithfulnessRetries; attempt++ {
		stepLabel := "summarize"
		if attempt > 0 {
			stepLabel = fmt.Sprintf("summarize-%d", attempt+1)
		}
		var primaryModelOverride *string
		var fallbackModelOverride *string
		if userModelSettings != nil {
			primaryModelOverride = ptrStringOrNil(userModelSettings.SummaryModel)
			fallbackModelOverride = ptrStringOrNil(userModelSettings.SummaryFallbackModel)
		}
		summaryAttempt, err := step.Run(ctx, stepLabel, func(ctx context.Context) (*processSummaryAttemptResult, error) {
			log.Printf("process-item summarize start item_id=%s attempt=%d", itemID, attempt+1)
			runtime, err := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, primaryModelOverride, "summary")
			if err != nil {
				return nil, err
			}
			sourceChars := len(sourceContent)
			workerCtx := service.WithWorkerTraceMetadata(ctx, "summary", userIDPtr, &data.SourceID, &itemID, nil)
			resp, err := deps.worker.SummarizeWithModel(workerCtx, titleForLLM, facts, &sourceChars, runtime.AnthropicKey, runtime.GoogleKey, runtime.GroqKey, runtime.DeepSeekKey, runtime.AlibabaKey, runtime.MistralKey, runtime.XAIKey, runtime.ZAIKey, runtime.FireworksKey, runtime.OpenAIKey, runtime.Model)
			if err != nil {
				return nil, err
			}
			return &processSummaryAttemptResult{
				Summary: resp,
				Runtime: runtime,
			}, nil
		})
		if err != nil {
			var failedModel *string
			if summaryAttempt != nil && summaryAttempt.Runtime != nil {
				failedModel = summaryAttempt.Runtime.Model
			}
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "summary", failedModel, attempt, userIDPtr, &data.SourceID, &itemID, nil, err)
			if canUseLLMFallback(primaryModelOverride, fallbackModelOverride, err) {
				fallbackStepLabel := stepLabel + "-fallback"
				fallbackAttempt, fallbackErr := step.Run(ctx, fallbackStepLabel, func(ctx context.Context) (*processSummaryAttemptResult, error) {
					log.Printf("process-item summarize fallback start item_id=%s attempt=%d", itemID, attempt+1)
					runtime, runtimeErr := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, fallbackModelOverride, "summary")
					if runtimeErr != nil {
						return nil, runtimeErr
					}
					sourceChars := len(sourceContent)
					workerCtx := service.WithWorkerTraceMetadata(ctx, "summary", userIDPtr, &data.SourceID, &itemID, nil)
					resp, workerErr := deps.worker.SummarizeWithModel(workerCtx, titleForLLM, facts, &sourceChars, runtime.AnthropicKey, runtime.GoogleKey, runtime.GroqKey, runtime.DeepSeekKey, runtime.AlibabaKey, runtime.MistralKey, runtime.XAIKey, runtime.ZAIKey, runtime.FireworksKey, runtime.OpenAIKey, runtime.Model)
					if workerErr != nil {
						return nil, workerErr
					}
					return &processSummaryAttemptResult{Summary: resp, Runtime: runtime}, nil
				})
				if fallbackErr != nil {
					var fallbackFailedModel *string
					if fallbackAttempt != nil && fallbackAttempt.Runtime != nil {
						fallbackFailedModel = fallbackAttempt.Runtime.Model
					}
					recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "summary", fallbackFailedModel, attempt, userIDPtr, &data.SourceID, &itemID, nil, fallbackErr)
					return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "summarize", fallbackErr)
				}
				summaryAttempt = fallbackAttempt
			} else {
				return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "summarize", err)
			}
		}

		summary = summaryAttempt.Summary
		summary.Summary = strings.TrimSpace(summary.Summary)
		recordLLMUsage(ctx, deps.llmUsageRepo, "summary", summary.LLM, userIDPtr, &data.SourceID, &itemID, nil)
		if summary.Summary == "" {
			err := fmt.Errorf("empty summary returned from worker")
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "summary", summaryAttempt.Runtime.Model, attempt, userIDPtr, &data.SourceID, &itemID, nil, err)
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "summarize", err)
		}
		recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, "summary", summary.LLM, attempt, userIDPtr, &data.SourceID, &itemID, nil)

		var faithfulnessModel *string
		if userModelSettings != nil {
			faithfulnessModel = ptrStringOrNil(userModelSettings.FaithfulnessCheckModel)
		}
		faithfulness, shouldRetry, err := executeLLMCheck(ctx, deps, llmCheckConfig[service.SummaryFaithfulnessResponse]{
			baseStepName:   "check-summary-faithfulness",
			purpose:        "faithfulness_check",
			resolvePurpose: "summary",
			attempt:        attempt,
			userID:         userIDPtr,
			sourceID:       &data.SourceID,
			itemID:         &itemID,
			modelOverride:  faithfulnessModel,
			defaultRuntime: summaryAttempt.Runtime,
			call: func(runtime *llmRuntime) (*service.SummaryFaithfulnessResponse, error) {
				workerCtx := service.WithWorkerTraceMetadata(ctx, "faithfulness_check", userIDPtr, &data.SourceID, &itemID, nil)
				return deps.worker.CheckSummaryFaithfulnessWithModel(
					workerCtx,
					titleForLLM,
					facts,
					summary.Summary,
					runtime.AnthropicKey,
					runtime.GoogleKey,
					runtime.GroqKey,
					runtime.DeepSeekKey,
					runtime.AlibabaKey,
					runtime.MistralKey,
					runtime.XAIKey,
					runtime.ZAIKey,
					runtime.FireworksKey,
					runtime.OpenAIKey,
					runtime.Model,
				)
			},
			getLLM:     func(result *service.SummaryFaithfulnessResponse) *service.LLMUsage { return result.LLM },
			getVerdict: func(result *service.SummaryFaithfulnessResponse) string { return result.Verdict },
		})
		if err != nil {
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "faithfulness check", err)
		}

		finalFaithfulness = faithfulness
		if !shouldRetry || attempt >= maxSummaryFaithfulnessRetries {
			summaryRetryCount = attempt
			break
		}
		log.Printf("process-item faithfulness retry item_id=%s attempt=%d verdict=%s comment=%s", itemID, attempt+1, faithfulness.Verdict, faithfulness.ShortComment)
	}

	if summary == nil {
		return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "summarize", fmt.Errorf("summary generation produced no result"))
	}
	if finalFaithfulness == nil {
		return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "faithfulness check", fmt.Errorf("faithfulness check produced no result"))
	}
	var faithfulnessComment *string
	if comment := strings.TrimSpace(finalFaithfulness.ShortComment); comment != "" {
		faithfulnessComment = &comment
	}
	if err := deps.itemRepo.UpsertSummaryFaithfulnessCheck(ctx, itemID, finalFaithfulness.Verdict, summaryRetryCount, faithfulnessComment); err != nil {
		return nil, fmt.Errorf("upsert summary faithfulness check: %w", err)
	}
	if finalFaithfulness.Verdict == "fail" {
		return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "faithfulness check", fmt.Errorf("%s", finalFaithfulness.ShortComment))
	}
	if err := deps.itemRepo.InsertSummary(
		ctx,
		itemID,
		summary.Summary,
		summary.Topics,
		summary.TranslatedTitle,
		summary.Score,
		summary.ScoreBreakdown,
		summary.ScoreReason,
		summary.ScorePolicyVersion,
	); err != nil {
		return nil, fmt.Errorf("insert summary: %w", err)
	}
	log.Printf("process-item summarize done item_id=%s topics=%d score=%.3f retries=%d faithfulness=%s", itemID, len(summary.Topics), summary.Score, summaryRetryCount, finalFaithfulness.Verdict)

	return &processSummaryStageResult{
		Summary:    summary,
		Check:      finalFaithfulness,
		RetryCount: summaryRetryCount,
	}, nil
}

func sendPickNotificationIfNeeded(
	ctx context.Context,
	deps processItemDeps,
	itemID string,
	url string,
	userIDPtr *string,
	titleForLLM *string,
	summary *service.SummarizeResponse,
) {
	if deps.oneSignal == nil || !deps.oneSignal.Enabled() || userIDPtr == nil || *userIDPtr == "" {
		return
	}
	rule := &model.NotificationPriorityRule{
		Sensitivity:      "medium",
		DailyCap:         deps.pickMaxPerDay,
		ThemeWeight:      1,
		ImmediateEnabled: true,
		GoalMatchEnabled: true,
	}
	if deps.notificationRepo != nil {
		if next, err := deps.notificationRepo.EnsureDefaults(ctx, *userIDPtr); err == nil && next != nil {
			rule = next
		}
	}
	var matchedGoals []model.ReadingGoal
	if deps.itemViewRepo != nil && deps.readingGoalRepo != nil {
		if detail, err := deps.itemViewRepo.GetDetail(ctx, itemID, *userIDPtr); err == nil && detail != nil {
			if goals, goalErr := deps.readingGoalRepo.ListByUser(ctx, *userIDPtr); goalErr == nil {
				matchedGoals = service.MatchReadingGoals(detail.Item, goals)
			}
		}
	}
	kind := "pick_update"
	if len(matchedGoals) > 0 {
		kind = "goal_match"
	}
	if kind == "pick_update" && summary.Score < deps.pickScoreThreshold {
		return
	}
	if (kind == "pick_update" && !rule.ImmediateEnabled) || (kind == "goal_match" && !rule.GoalMatchEnabled) {
		return
	}
	alreadyNotified, err := deps.pushLogRepo.ExistsByUserKindItem(ctx, *userIDPtr, kind, itemID)
	if err != nil || alreadyNotified {
		if err != nil {
			log.Printf("process-item pick-notify exists failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
		}
		return
	}
	dayJST := timeutil.StartOfDayJST(timeutil.NowJST())
	countToday, err := deps.pushLogRepo.CountByUserKindsDay(ctx, *userIDPtr, []string{"pick_update", "goal_match"}, dayJST)
	if err != nil || countToday >= deps.pickMaxPerDay {
		if err != nil {
			log.Printf("process-item pick-notify count failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
		}
		return
	}
	decision := service.RouteNotificationPriority(service.NotificationPriorityInput{
		ItemScore:           summary.Score,
		GoalMatch:           len(matchedGoals) > 0,
		RecentNotifications: countToday,
		DuplicateDigestRisk: false,
		Sensitivity:         rule.Sensitivity,
		DailyCap:            rule.DailyCap,
		ThemeWeight:         rule.ThemeWeight,
	})
	if decision.Route == "suppress" || (decision.Route != "send_now" && len(matchedGoals) == 0) {
		return
	}
	u, err := deps.userRepo.GetByID(ctx, *userIDPtr)
	if err != nil {
		log.Printf("process-item pick-notify user lookup failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
		return
	}
	title := "Sifto: 注目記事を追加しました"
	itemTitle := strings.TrimSpace(summary.TranslatedTitle)
	if itemTitle == "" {
		itemTitle = strings.TrimSpace(coalescePtrStr(titleForLLM, url))
	}
	message := itemTitle
	if len(matchedGoals) > 0 {
		title = "Sifto: 読書ゴールに一致する新着があります"
		message = fmt.Sprintf("「%s」に一致: %s", matchedGoals[0].Title, itemTitle)
	}
	if len(message) > 120 {
		message = message[:120]
	}
	pushRes, err := deps.oneSignal.SendToExternalID(
		ctx,
		u.Email,
		title,
		message,
		appPageURL("/items/"+itemID),
		map[string]any{
			"type":     kind,
			"item_id":  itemID,
			"item_url": appPageURL("/items/" + itemID),
			"url":      url,
			"score":    summary.Score,
			"reason":   decision.Reason,
		},
	)
	if err != nil {
		log.Printf("process-item pick-notify send failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
		return
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
	if err := deps.pushLogRepo.Insert(ctx, repository.PushNotificationLogInput{
		UserID:                  *userIDPtr,
		Kind:                    kind,
		ItemID:                  &itemID,
		DayJST:                  dayJST,
		Title:                   title,
		Message:                 message,
		OneSignalNotificationID: oneSignalID,
		Recipients:              recipients,
	}); err != nil {
		log.Printf("process-item pick-notify log failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
	}
}

func createEmbeddingIfPossible(
	ctx context.Context,
	deps processItemDeps,
	data processItemEventData,
	itemID string,
	userIDPtr *string,
	userModelSettings *model.UserSettings,
	titleForLLM *string,
	summary *service.SummarizeResponse,
	facts []string,
) {
	userOpenAIKey, err := loadUserOpenAIAPIKey(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr)
	if err != nil {
		log.Printf("process-item embedding skip item_id=%s reason=%v", itemID, err)
		return
	}
	inputText := buildItemEmbeddingInput(titleForLLM, summary.Summary, summary.Topics, facts)
	embModel := service.OpenAIEmbeddingModel()
	if userModelSettings != nil && userModelSettings.EmbeddingModel != nil && service.IsSupportedOpenAIEmbeddingModel(*userModelSettings.EmbeddingModel) {
		embModel = *userModelSettings.EmbeddingModel
	}
	embResp, err := step.Run(ctx, "create-embedding", func(ctx context.Context) (*service.CreateEmbeddingResponse, error) {
		log.Printf("process-item create-embedding start item_id=%s model=%s", itemID, embModel)
		return deps.openAI.CreateEmbedding(ctx, *userOpenAIKey, embModel, inputText)
	})
	if err != nil {
		recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "embedding", &embModel, 0, userIDPtr, &data.SourceID, &itemID, nil, err)
		log.Printf("process-item create-embedding failed item_id=%s err=%v", itemID, err)
		return
	}
	if err := deps.itemRepo.UpsertEmbedding(ctx, itemID, embModel, embResp.Embedding); err != nil {
		log.Printf("process-item upsert-embedding failed item_id=%s err=%v", itemID, err)
		return
	}
	recordLLMUsage(ctx, deps.llmUsageRepo, "embedding", embResp.LLM, userIDPtr, &data.SourceID, &itemID, nil)
	recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, "embedding", embResp.LLM, 0, userIDPtr, &data.SourceID, &itemID, nil)
	log.Printf("process-item create-embedding done item_id=%s dims=%d", itemID, len(embResp.Embedding))
}

func updateItemAfterExtract(
	ctx context.Context,
	itemRepo *repository.ItemInngestRepo,
	itemID string,
	extracted *service.ExtractBodyResponse,
) error {
	var publishedAt *time.Time
	if extracted.PublishedAt != nil {
		t, err := timeutil.ParseToJST(*extracted.PublishedAt)
		if err == nil {
			publishedAt = &t
		}
	}
	return itemRepo.UpdateAfterExtract(ctx, itemID, extracted.Content, extracted.Title, extracted.ImageURL, publishedAt)
}
