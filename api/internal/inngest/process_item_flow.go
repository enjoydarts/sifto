package inngest

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/inngest/inngestgo/step"
	"github.com/minoru-kitayama/sifto/api/internal/model"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
	"github.com/minoru-kitayama/sifto/api/internal/service"
	"github.com/minoru-kitayama/sifto/api/internal/timeutil"
)

type processItemEventData struct {
	ItemID   string `json:"item_id"`
	SourceID string `json:"source_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
}

type processItemDeps struct {
	itemRepo           *repository.ItemInngestRepo
	llmUsageRepo       *repository.LLMUsageLogRepo
	llmExecutionRepo   *repository.LLMExecutionEventRepo
	sourceRepo         *repository.SourceRepo
	userSettingsRepo   *repository.UserSettingsRepo
	userRepo           *repository.UserRepo
	pushLogRepo        *repository.PushNotificationLogRepo
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
		factsAttempt, err := step.Run(ctx, stepLabel, func(ctx context.Context) (*processFactsAttemptResult, error) {
			log.Printf("process-item extract-facts start item_id=%s attempt=%d", itemID, attempt+1)
			var modelOverride *string
			if userModelSettings != nil {
				modelOverride = ptrStringOrNil(userModelSettings.FactsModel)
			}
			runtime, err := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, modelOverride, "facts")
			if err != nil {
				return nil, err
			}
			resp, err := deps.worker.ExtractFactsWithModel(ctx, titleForLLM, content, runtime.AnthropicKey, runtime.GoogleKey, runtime.GroqKey, runtime.DeepSeekKey, runtime.OpenAIKey, runtime.Model)
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
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "extract facts", err)
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

		checkStep := "check-facts"
		if attempt > 0 {
			checkStep = fmt.Sprintf("check-facts-%d", attempt+1)
		}
		factsCheck, err := step.Run(ctx, checkStep, func(ctx context.Context) (*service.FactsCheckResponse, error) {
			var modelOverride *string
			if userModelSettings != nil {
				modelOverride = ptrStringOrNil(userModelSettings.FactsCheckModel)
			}
			var resp *service.FactsCheckResponse
			if modelOverride == nil || strings.TrimSpace(*modelOverride) == "" {
				modelOverride = factsAttempt.Runtime.Model
				resp, err = deps.worker.CheckFactsWithModel(
					ctx,
					titleForLLM,
					content,
					factsResp.Facts,
					factsAttempt.Runtime.AnthropicKey,
					factsAttempt.Runtime.GoogleKey,
					factsAttempt.Runtime.GroqKey,
					factsAttempt.Runtime.DeepSeekKey,
					factsAttempt.Runtime.OpenAIKey,
					modelOverride,
				)
			} else {
				runtime, keyErr := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, modelOverride, "facts")
				if keyErr != nil {
					return nil, keyErr
				}
				resp, err = deps.worker.CheckFactsWithModel(ctx, titleForLLM, content, factsResp.Facts, runtime.AnthropicKey, runtime.GoogleKey, runtime.GroqKey, runtime.DeepSeekKey, runtime.OpenAIKey, runtime.Model)
			}
			if err != nil {
				return nil, err
			}
			if resp == nil {
				return nil, fmt.Errorf("facts check returned nil response")
			}
			recordLLMUsage(ctx, deps.llmUsageRepo, "facts_check", resp.LLM, userIDPtr, &data.SourceID, &itemID, nil)
			return resp, nil
		})
		if err != nil {
			var failedModel *string
			if userModelSettings != nil && userModelSettings.FactsCheckModel != nil && strings.TrimSpace(*userModelSettings.FactsCheckModel) != "" {
				failedModel = userModelSettings.FactsCheckModel
			} else {
				failedModel = factsAttempt.Runtime.Model
			}
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "facts_check", failedModel, attempt, userIDPtr, &data.SourceID, &itemID, nil, err)
			finalFactsCheck = fallbackFactsCheckWarning(err)
			factsRetryCount = attempt
			log.Printf("process-item facts_check fallback-warn item_id=%s attempt=%d err=%v", itemID, attempt+1, err)
			break
		}

		recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, "facts_check", factsCheck.LLM, attempt, userIDPtr, &data.SourceID, &itemID, nil)
		finalFactsCheck = factsCheck
		if factsCheck.Verdict != "fail" || attempt >= maxFactsCheckRetries {
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
		summaryAttempt, err := step.Run(ctx, stepLabel, func(ctx context.Context) (*processSummaryAttemptResult, error) {
			log.Printf("process-item summarize start item_id=%s attempt=%d", itemID, attempt+1)
			var modelOverride *string
			if userModelSettings != nil {
				modelOverride = ptrStringOrNil(userModelSettings.SummaryModel)
			}
			runtime, err := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, modelOverride, "summary")
			if err != nil {
				return nil, err
			}
			sourceChars := len(sourceContent)
			resp, err := deps.worker.SummarizeWithModel(ctx, titleForLLM, facts, &sourceChars, runtime.AnthropicKey, runtime.GoogleKey, runtime.GroqKey, runtime.DeepSeekKey, runtime.OpenAIKey, runtime.Model)
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
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "summarize", err)
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

		faithfulnessStep := "check-summary-faithfulness"
		if attempt > 0 {
			faithfulnessStep = fmt.Sprintf("check-summary-faithfulness-%d", attempt+1)
		}
		faithfulness, err := step.Run(ctx, faithfulnessStep, func(ctx context.Context) (*service.SummaryFaithfulnessResponse, error) {
			var modelOverride *string
			if userModelSettings != nil {
				modelOverride = ptrStringOrNil(userModelSettings.FaithfulnessCheckModel)
			}
			var resp *service.SummaryFaithfulnessResponse
			if modelOverride == nil || strings.TrimSpace(*modelOverride) == "" {
				modelOverride = summaryAttempt.Runtime.Model
				resp, err = deps.worker.CheckSummaryFaithfulnessWithModel(
					ctx,
					titleForLLM,
					facts,
					summary.Summary,
					summaryAttempt.Runtime.AnthropicKey,
					summaryAttempt.Runtime.GoogleKey,
					summaryAttempt.Runtime.GroqKey,
					summaryAttempt.Runtime.DeepSeekKey,
					summaryAttempt.Runtime.OpenAIKey,
					modelOverride,
				)
			} else {
				runtime, keyErr := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, userIDPtr, modelOverride, "summary")
				if keyErr != nil {
					return nil, keyErr
				}
				resp, err = deps.worker.CheckSummaryFaithfulnessWithModel(ctx, titleForLLM, facts, summary.Summary, runtime.AnthropicKey, runtime.GoogleKey, runtime.GroqKey, runtime.DeepSeekKey, runtime.OpenAIKey, runtime.Model)
			}
			if err != nil {
				return nil, err
			}
			recordLLMUsage(ctx, deps.llmUsageRepo, "faithfulness_check", resp.LLM, userIDPtr, &data.SourceID, &itemID, nil)
			return resp, nil
		})
		if err != nil {
			var failedModel *string
			if userModelSettings != nil && userModelSettings.FaithfulnessCheckModel != nil && strings.TrimSpace(*userModelSettings.FaithfulnessCheckModel) != "" {
				failedModel = userModelSettings.FaithfulnessCheckModel
			} else {
				failedModel = summaryAttempt.Runtime.Model
			}
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, "faithfulness_check", failedModel, attempt, userIDPtr, &data.SourceID, &itemID, nil, err)
			return nil, markProcessItemFailed(ctx, deps.itemRepo, itemID, "faithfulness check", err)
		}

		recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, "faithfulness_check", faithfulness.LLM, attempt, userIDPtr, &data.SourceID, &itemID, nil)
		finalFaithfulness = faithfulness
		if faithfulness.Verdict != "fail" || attempt >= maxSummaryFaithfulnessRetries {
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
	if deps.oneSignal == nil || !deps.oneSignal.Enabled() || userIDPtr == nil || *userIDPtr == "" || summary.Score < deps.pickScoreThreshold {
		return
	}

	alreadyNotified, err := deps.pushLogRepo.ExistsByUserKindItem(ctx, *userIDPtr, "pick_update", itemID)
	if err != nil || alreadyNotified {
		if err != nil {
			log.Printf("process-item pick-notify exists failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
		}
		return
	}
	dayJST := timeutil.StartOfDayJST(timeutil.NowJST())
	countToday, err := deps.pushLogRepo.CountByUserKindDay(ctx, *userIDPtr, "pick_update", dayJST)
	if err != nil || countToday >= deps.pickMaxPerDay {
		if err != nil {
			log.Printf("process-item pick-notify count failed item_id=%s user_id=%s err=%v", itemID, *userIDPtr, err)
		}
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
			"type":     "pick_update",
			"item_id":  itemID,
			"item_url": appPageURL("/items/" + itemID),
			"url":      url,
			"score":    summary.Score,
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
		Kind:                    "pick_update",
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
