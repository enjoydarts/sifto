package inngest

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

func composeDigestEmailCopy(
	ctx context.Context,
	digestRepo *repository.DigestInngestRepo,
	itemRepo *repository.ItemRepo,
	userSettingsRepo *repository.UserSettingsRepo,
	llmUsageRepo *repository.LLMUsageLogRepo,
	llmExecutionRepo *repository.LLMExecutionEventRepo,
	workerDeps processItemDeps,
	data DigestCreatedData,
	digest *model.DigestDetail,
	userModelSettings *model.UserSettings,
) error {
	const maxDigestClusterDraftRetries = 2
	const maxDigestRetries = 2

	log.Printf("compose-digest-copy step-exec digest_id=%s", data.DigestID)
	clusterItems := make([]model.Item, 0, len(digest.Items))
	for _, di := range digest.Items {
		it := di.Item
		it.SummaryScore = di.Summary.Score
		it.SummaryTopics = di.Summary.Topics
		clusterItems = append(clusterItems, it)
	}
	embClusters, err := itemRepo.ClusterItemsByEmbeddings(ctx, clusterItems)
	if err != nil {
		return fmt.Errorf("cluster digest items: %w", err)
	}
	drafts := buildDigestClusterDrafts(digest.Items, embClusters)
	drafts = compressDigestClusterDrafts(drafts, 20)

	var clusterDraftModel *string
	if userModelSettings != nil {
		clusterDraftModel = ptrStringOrNil(userModelSettings.DigestClusterModel)
	}
	clusterDraftRuntime, keyErr := resolveLLMRuntime(ctx, userSettingsRepo, workerDeps.secretCipher, &data.UserID, clusterDraftModel, "digest_cluster_draft")
	if keyErr != nil {
		return keyErr
	}

	totalClusterDraftRetryCount := 0
	for i := range drafts {
		sourceLines := draftSourceLines(drafts[i].DraftSummary)
		if len(sourceLines) == 0 {
			continue
		}
		valid := false
		for attempt := 0; attempt <= maxDigestClusterDraftRetries; attempt++ {
			workerCtx := service.WithWorkerTraceMetadata(ctx, "digest_cluster_draft", &data.UserID, nil, nil, &data.DigestID)
			resp, err := workerDeps.worker.ComposeDigestClusterDraftWithModel(
				workerCtx,
				drafts[i].ClusterLabel,
				drafts[i].ItemCount,
				drafts[i].Topics,
				sourceLines,
				clusterDraftRuntime.AnthropicKey,
				clusterDraftRuntime.GoogleKey,
				clusterDraftRuntime.GroqKey,
				clusterDraftRuntime.DeepSeekKey,
				clusterDraftRuntime.AlibabaKey,
				clusterDraftRuntime.MistralKey,
				clusterDraftRuntime.XAIKey,
				clusterDraftRuntime.ZAIKey,
				clusterDraftRuntime.FireworksKey,
				clusterDraftRuntime.OpenAIKey,
				clusterDraftRuntime.Model,
			)
			if err != nil {
				recordLLMExecutionFailure(ctx, llmExecutionRepo, "digest_cluster_draft", clusterDraftRuntime.Model, attempt, &data.UserID, nil, nil, &data.DigestID, nil, err)
				return fmt.Errorf("compose digest cluster draft rank=%d attempt=%d: %w", drafts[i].Rank, attempt+1, err)
			}
			if resp != nil {
				recordLLMUsage(ctx, llmUsageRepo, "digest_cluster_draft", resp.LLM, &data.UserID, nil, nil, &data.DigestID, nil)
			}
			candidate := drafts[i].DraftSummary
			if resp != nil && strings.TrimSpace(resp.DraftSummary) != "" {
				candidate = resp.DraftSummary
			}
			if err := validateDigestClusterDraftCompletion(candidate); err == nil {
				drafts[i].DraftSummary = candidate
				if resp != nil {
					recordLLMExecutionSuccess(ctx, llmExecutionRepo, "digest_cluster_draft", resp.LLM, attempt, &data.UserID, nil, nil, &data.DigestID, nil)
				}
				totalClusterDraftRetryCount += attempt
				valid = true
				break
			} else if attempt >= maxDigestClusterDraftRetries {
				recordLLMExecutionFailure(ctx, llmExecutionRepo, "digest_cluster_draft", clusterDraftRuntime.Model, attempt, &data.UserID, nil, nil, &data.DigestID, nil, err)
				return fmt.Errorf("compose digest cluster draft rank=%d incomplete after %d retries: %w", drafts[i].Rank, attempt, err)
			} else {
				reason := digestClusterDraftValidationReason(candidate)
				lastLine := ""
				trimmed := strings.TrimSpace(candidate)
				if trimmed != "" {
					lines := strings.Split(trimmed, "\n")
					lastLine = strings.TrimSpace(lines[len(lines)-1])
				}
				lineCount := 0
				if trimmed != "" {
					for _, line := range strings.Split(trimmed, "\n") {
						if strings.TrimSpace(line) != "" {
							lineCount++
						}
					}
				}
				inputTokens, outputTokens := 0, 0
				modelName := ""
				if resp != nil && resp.LLM != nil {
					inputTokens = resp.LLM.InputTokens
					outputTokens = resp.LLM.OutputTokens
					modelName = strings.TrimSpace(resp.LLM.ResolvedModel)
					if modelName == "" {
						modelName = strings.TrimSpace(resp.LLM.Model)
					}
				}
				recordLLMExecutionFailure(ctx, llmExecutionRepo, "digest_cluster_draft", clusterDraftRuntime.Model, attempt, &data.UserID, nil, nil, &data.DigestID, nil, err)
				log.Printf(
					"compose-digest-copy cluster-draft retry digest_id=%s rank=%d attempt=%d reason=%s model=%s input_tokens=%d output_tokens=%d candidate_chars=%d line_count=%d last_line=%q err=%v",
					data.DigestID,
					drafts[i].Rank,
					attempt+1,
					reason,
					modelName,
					inputTokens,
					outputTokens,
					len([]rune(trimmed)),
					lineCount,
					lastLine,
					err,
				)
			}
		}
		if !valid {
			return fmt.Errorf("compose digest cluster draft rank=%d produced no valid draft", drafts[i].Rank)
		}
	}

	if err := digestRepo.ReplaceClusterDrafts(ctx, data.DigestID, drafts); err != nil {
		return fmt.Errorf("store digest cluster drafts: %w", err)
	}
	storedDrafts, err := digestRepo.ListClusterDrafts(ctx, data.DigestID)
	if err != nil {
		return fmt.Errorf("reload digest cluster drafts: %w", err)
	}
	items := buildComposeItemsFromClusterDrafts(storedDrafts, len(storedDrafts))
	log.Printf("compose-digest-copy compacted digest_id=%s source_items=%d cluster_drafts=%d compose_items=%d", data.DigestID, len(digest.Items), len(storedDrafts), len(items))

	var modelOverride *string
	if userModelSettings != nil {
		modelOverride = ptrStringOrNil(userModelSettings.DigestModel)
	}
	digestRuntime, keyErr := resolveLLMRuntime(ctx, userSettingsRepo, workerDeps.secretCipher, &data.UserID, modelOverride, "digest")
	if keyErr != nil {
		return keyErr
	}
	digestPromptResolution := service.ResolvePromptResolution(ctx, workerDeps.promptResolver, service.PromptResolveInput{
		PromptKey:      "digest.default",
		AssignmentUnit: "digest_id",
		AssignmentKey:  data.DigestID,
	})
	digestPromptConfig := service.WorkerPromptConfigFromResolution(digestPromptResolution)

	var resp *service.ComposeDigestResponse
	digestRetryCount := 0
	for attempt := 0; attempt <= maxDigestRetries; attempt++ {
		workerCtx := service.WithWorkerTraceMetadata(ctx, "digest", &data.UserID, nil, nil, &data.DigestID)
		resp, err = workerDeps.worker.ComposeDigestWithModel(workerCtx, digest.DigestDate, items, digestRuntime.AnthropicKey, digestRuntime.GoogleKey, digestRuntime.GroqKey, digestRuntime.DeepSeekKey, digestRuntime.AlibabaKey, digestRuntime.MistralKey, digestRuntime.XAIKey, digestRuntime.ZAIKey, digestRuntime.FireworksKey, digestRuntime.OpenAIKey, digestRuntime.Model, digestPromptConfig)
		if err != nil {
			recordLLMExecutionFailure(ctx, llmExecutionRepo, "digest", digestRuntime.Model, attempt, &data.UserID, nil, nil, &data.DigestID, digestPromptResolution, err)
			return err
		}
		recordLLMUsage(ctx, llmUsageRepo, "digest", resp.LLM, &data.UserID, nil, nil, &data.DigestID, digestPromptResolution)
		if err := validateDigestCompletion(resp.Subject, resp.Body); err == nil {
			recordLLMExecutionSuccess(ctx, llmExecutionRepo, "digest", resp.LLM, attempt, &data.UserID, nil, nil, &data.DigestID, digestPromptResolution)
			digestRetryCount = attempt
			break
		} else if attempt >= maxDigestRetries {
			recordLLMExecutionFailure(ctx, llmExecutionRepo, "digest", digestRuntime.Model, attempt, &data.UserID, nil, nil, &data.DigestID, digestPromptResolution, err)
			return fmt.Errorf("compose digest incomplete after %d retries: %w", attempt, err)
		} else {
			recordLLMExecutionFailure(ctx, llmExecutionRepo, "digest", digestRuntime.Model, attempt, &data.UserID, nil, nil, &data.DigestID, digestPromptResolution, err)
			log.Printf("compose-digest-copy digest retry digest_id=%s attempt=%d err=%v", data.DigestID, attempt+1, err)
		}
	}
	if resp == nil {
		return fmt.Errorf("compose digest returned no response")
	}
	resp.Subject = service.FormatDigestEmailSubject(digest.DigestDate, resp.Subject)
	if err := digestRepo.UpdateComposeRetryCounts(ctx, data.DigestID, digestRetryCount, totalClusterDraftRetryCount); err != nil {
		return fmt.Errorf("update digest retry counts: %w", err)
	}
	log.Printf("compose-digest-copy worker-done digest_id=%s subject_len=%d body_len=%d", data.DigestID, len(resp.Subject), len(resp.Body))
	if err := digestRepo.UpdateEmailCopy(ctx, data.DigestID, resp.Subject, resp.Body); err != nil {
		return err
	}
	return nil
}
