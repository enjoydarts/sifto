package inngest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
)

var llmUsageCache service.JSONCache

type llmExecutionTriggerContextKey struct{}

type llmExecutionTrigger struct {
	TriggerID     string
	TriggerReason string
}

func withLLMExecutionTrigger(ctx context.Context, triggerID, triggerReason string) context.Context {
	if strings.TrimSpace(triggerID) == "" {
		return ctx
	}
	return context.WithValue(ctx, llmExecutionTriggerContextKey{}, llmExecutionTrigger{
		TriggerID:     strings.TrimSpace(triggerID),
		TriggerReason: strings.TrimSpace(triggerReason),
	})
}

func llmExecutionTriggerFromContext(ctx context.Context) *llmExecutionTrigger {
	v, ok := ctx.Value(llmExecutionTriggerContextKey{}).(llmExecutionTrigger)
	if !ok || strings.TrimSpace(v.TriggerID) == "" {
		return nil
	}
	return &v
}

func recordLLMUsage(ctx context.Context, repo *repository.LLMUsageLogRepo, purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string, prompt *service.PromptResolution) {
	usage = service.NormalizeCatalogPricedUsage(purpose, usage)
	if repo == nil || usage == nil {
		return
	}
	storedModel := llmUsageStoredModel(usage)
	if usage.Provider == "" || storedModel == "" {
		return
	}
	idempotencyKey := llmUsageIdempotencyKey(purpose, usage, userID, sourceID, itemID, digestID, prompt)
	pricingSource := usage.PricingSource
	if pricingSource == "" {
		pricingSource = "unknown"
	}
	if err := repo.Insert(ctx, repository.LLMUsageLogInput{
		IdempotencyKey:           &idempotencyKey,
		UserID:                   userID,
		SourceID:                 sourceID,
		ItemID:                   itemID,
		DigestID:                 digestID,
		PromptKey:                promptKey(prompt),
		PromptSource:             promptSource(prompt),
		PromptVersionID:          promptVersionID(prompt),
		PromptVersionNumber:      promptVersionNumber(prompt),
		PromptExperimentID:       nil,
		PromptExperimentArmID:    nil,
		Provider:                 usage.Provider,
		Model:                    storedModel,
		RequestedModel:           strings.TrimSpace(usage.RequestedModel),
		ResolvedModel:            strings.TrimSpace(usage.ResolvedModel),
		PricingModelFamily:       usage.PricingModelFamily,
		PricingSource:            pricingSource,
		OpenRouterCostUSD:        usage.OpenRouterCostUSD,
		OpenRouterGenerationID:   strings.TrimSpace(usage.OpenRouterGenerationID),
		Purpose:                  purpose,
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		EstimatedCostUSD:         usage.EstimatedCostUSD,
	}); err != nil {
		log.Printf("record llm usage purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, *userID)
	}
}

func llmUsageStoredModel(usage *service.LLMUsage) string {
	if usage == nil {
		return ""
	}
	if strings.TrimSpace(usage.Provider) == "openrouter" && strings.TrimSpace(usage.ResolvedModel) != "" {
		return strings.TrimSpace(usage.ResolvedModel)
	}
	return strings.TrimSpace(usage.Model)
}

func recordLLMExecutionSuccess(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, usage *service.LLMUsage, attemptIndex int, userID, sourceID, itemID, digestID *string, prompt *service.PromptResolution) {
	if repo == nil || usage == nil {
		return
	}
	if usage.Provider == "" || usage.Model == "" {
		return
	}
	var idempotencyKey *string
	var triggerID *string
	var triggerReason *string
	if trigger := llmExecutionTriggerFromContext(ctx); trigger != nil {
		key := llmExecutionEventIdempotencyKey(repository.LLMExecutionEventInput{
			UserID:              userID,
			SourceID:            sourceID,
			ItemID:              itemID,
			DigestID:            digestID,
			PromptKey:           promptKey(prompt),
			PromptSource:        promptSource(prompt),
			PromptVersionID:     promptVersionID(prompt),
			PromptVersionNumber: promptVersionNumber(prompt),
			TriggerID:           &trigger.TriggerID,
			TriggerReason:       triggerReason,
			Provider:            usage.Provider,
			Model:               usage.Model,
			Purpose:             purpose,
			Status:              "success",
			AttemptIndex:        attemptIndex,
		})
		idempotencyKey = &key
		triggerID = &trigger.TriggerID
		if strings.TrimSpace(trigger.TriggerReason) != "" {
			v := strings.TrimSpace(trigger.TriggerReason)
			triggerReason = &v
		}
	}
	if err := repo.Insert(ctx, repository.LLMExecutionEventInput{
		IdempotencyKey:      idempotencyKey,
		UserID:              userID,
		SourceID:            sourceID,
		ItemID:              itemID,
		DigestID:            digestID,
		PromptKey:           promptKey(prompt),
		PromptSource:        promptSource(prompt),
		PromptVersionID:     promptVersionID(prompt),
		PromptVersionNumber: promptVersionNumber(prompt),
		TriggerID:           triggerID,
		TriggerReason:       triggerReason,
		Provider:            usage.Provider,
		Model:               usage.Model,
		Purpose:             purpose,
		Status:              "success",
		AttemptIndex:        attemptIndex,
	}); err != nil {
		log.Printf("record llm execution success purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, *userID)
	}
}

func recordLLMExecutionFailure(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, model *string, attemptIndex int, userID, sourceID, itemID, digestID *string, prompt *service.PromptResolution, err error) {
	if repo == nil || model == nil || strings.TrimSpace(*model) == "" || err == nil {
		return
	}
	modelVal := strings.TrimSpace(*model)
	provider := service.LLMProviderForModel(&modelVal)
	errorKind, emptyResponse := classifyLLMExecutionError(err)
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	var idempotencyKey *string
	var triggerID *string
	var triggerReason *string
	if trigger := llmExecutionTriggerFromContext(ctx); trigger != nil {
		key := llmExecutionEventIdempotencyKey(repository.LLMExecutionEventInput{
			UserID:              userID,
			SourceID:            sourceID,
			ItemID:              itemID,
			DigestID:            digestID,
			PromptKey:           promptKey(prompt),
			PromptSource:        promptSource(prompt),
			PromptVersionID:     promptVersionID(prompt),
			PromptVersionNumber: promptVersionNumber(prompt),
			TriggerID:           &trigger.TriggerID,
			TriggerReason:       triggerReason,
			Provider:            provider,
			Model:               modelVal,
			Purpose:             purpose,
			Status:              "failure",
			AttemptIndex:        attemptIndex,
			EmptyResponse:       emptyResponse,
			ErrorKind:           &errorKind,
			ErrorMessage:        &message,
		})
		idempotencyKey = &key
		triggerID = &trigger.TriggerID
		if strings.TrimSpace(trigger.TriggerReason) != "" {
			v := strings.TrimSpace(trigger.TriggerReason)
			triggerReason = &v
		}
	}
	if err := repo.Insert(ctx, repository.LLMExecutionEventInput{
		IdempotencyKey:      idempotencyKey,
		UserID:              userID,
		SourceID:            sourceID,
		ItemID:              itemID,
		DigestID:            digestID,
		PromptKey:           promptKey(prompt),
		PromptSource:        promptSource(prompt),
		PromptVersionID:     promptVersionID(prompt),
		PromptVersionNumber: promptVersionNumber(prompt),
		TriggerID:           triggerID,
		TriggerReason:       triggerReason,
		Provider:            provider,
		Model:               modelVal,
		Purpose:             purpose,
		Status:              "failure",
		AttemptIndex:        attemptIndex,
		EmptyResponse:       emptyResponse,
		ErrorKind:           &errorKind,
		ErrorMessage:        &message,
	}); err != nil {
		log.Printf("record llm execution failure purpose=%s: %v", purpose, err)
		return
	}
	if userID != nil {
		_ = service.BumpUserLLMUsageCacheVersion(ctx, llmUsageCache, toVal(userID))
	}
}

func recordLLMExecutionFailuresFromUsage(ctx context.Context, repo *repository.LLMExecutionEventRepo, purpose string, usage *service.LLMUsage, attemptIndex int, userID, sourceID, itemID, digestID *string, prompt *service.PromptResolution) {
	if repo == nil || usage == nil || len(usage.ExecutionFailures) == 0 {
		return
	}
	for _, failure := range usage.ExecutionFailures {
		model := strings.TrimSpace(failure.Model)
		if model == "" {
			continue
		}
		reason := strings.TrimSpace(failure.Reason)
		if reason == "" {
			reason = "worker internal fallback"
		}
		recordLLMExecutionFailure(ctx, repo, purpose, &model, attemptIndex, userID, sourceID, itemID, digestID, prompt, fmt.Errorf("%s", reason))
	}
}

func llmExecutionEventIdempotencyKey(in repository.LLMExecutionEventInput) string {
	raw := fmt.Sprintf(
		"trigger=%s|reason=%s|purpose=%s|provider=%s|model=%s|status=%s|attempt=%d|u=%s|s=%s|i=%s|d=%s|pk=%s|ps=%s|pvid=%s|pvn=%s|empty=%t|ek=%s|em=%s",
		toVal(in.TriggerID),
		toVal(in.TriggerReason),
		in.Purpose,
		in.Provider,
		in.Model,
		in.Status,
		in.AttemptIndex,
		toVal(in.UserID),
		toVal(in.SourceID),
		toVal(in.ItemID),
		toVal(in.DigestID),
		in.PromptKey,
		in.PromptSource,
		toVal(in.PromptVersionID),
		toIntVal(in.PromptVersionNumber),
		in.EmptyResponse,
		toVal(in.ErrorKind),
		toVal(in.ErrorMessage),
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func toVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func toIntVal(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func classifyLLMExecutionError(err error) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(s, "response_snippet=(empty)"),
		strings.Contains(s, "returned nil response"),
		strings.Contains(s, "empty summary"),
		strings.Contains(s, "empty facts"),
		strings.Contains(s, "empty response"):
		return "empty_response", true
	case strings.Contains(s, "short_comment missing"),
		strings.Contains(s, "parse failed"),
		strings.Contains(s, "json"):
		return "parse_error", false
	case strings.Contains(s, "looks truncated"),
		strings.Contains(s, "incomplete after"),
		strings.Contains(s, "compose digest incomplete"):
		return "incomplete_output", false
	case strings.Contains(s, "timeout"),
		strings.Contains(s, "deadline exceeded"):
		return "timeout", false
	default:
		return "worker_error", false
	}
}

func llmUsageIdempotencyKey(purpose string, usage *service.LLMUsage, userID, sourceID, itemID, digestID *string, prompt *service.PromptResolution) string {
	model := llmUsageStoredModel(usage)
	raw := fmt.Sprintf(
		"purpose=%s|provider=%s|model=%s|u=%s|s=%s|i=%s|d=%s|pk=%s|ps=%s|pvid=%s|pvn=%s|in=%d|out=%d|cw=%d|cr=%d",
		purpose,
		usage.Provider,
		model,
		toVal(userID),
		toVal(sourceID),
		toVal(itemID),
		toVal(digestID),
		promptKey(prompt),
		promptSource(prompt),
		toVal(promptVersionID(prompt)),
		toIntVal(promptVersionNumber(prompt)),
		usage.InputTokens,
		usage.OutputTokens,
		usage.CacheCreationInputTokens,
		usage.CacheReadInputTokens,
	)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func promptKey(prompt *service.PromptResolution) string {
	if prompt == nil {
		return ""
	}
	return strings.TrimSpace(prompt.PromptKey)
}

func promptSource(prompt *service.PromptResolution) string {
	if prompt == nil {
		return ""
	}
	return strings.TrimSpace(prompt.PromptSource)
}

func promptVersionID(prompt *service.PromptResolution) *string {
	if prompt == nil || prompt.PromptVersionID == nil || strings.TrimSpace(*prompt.PromptVersionID) == "" {
		return nil
	}
	return prompt.PromptVersionID
}

func promptVersionNumber(prompt *service.PromptResolution) *int {
	if prompt == nil || prompt.PromptVersionNumber == nil {
		return nil
	}
	return prompt.PromptVersionNumber
}
