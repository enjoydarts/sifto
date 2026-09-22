package inngest

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/inngest/inngestgo/step"
)

type llmCheckConfig[T any] struct {
	baseStepName     string
	purpose          string
	resolvePurpose   string
	attempt          int
	userID           *string
	sourceID         *string
	itemID           *string
	modelOverride    *string
	fallbackModel    *string
	defaultRuntime   *llmRuntime
	call             func(runtime *llmRuntime) (*T, error)
	getLLM           func(result *T) *service.LLMUsage
	getVerdict       func(result *T) string
	onExecutionError func(err error) *T
}

func executeLLMCheck[T any](ctx context.Context, deps processItemDeps, cfg llmCheckConfig[T]) (*T, bool, error) {
	stepName := cfg.baseStepName
	if cfg.attempt > 0 {
		stepName = fmt.Sprintf("%s-%d", cfg.baseStepName, cfg.attempt+1)
	}

	var attemptRuntime *llmRuntime
	result, err := step.Run(ctx, stepName, func(ctx context.Context) (*T, error) {
		runtime := cfg.defaultRuntime
		if chooseModelOverride(cfg.modelOverride, nil) != nil {
			resolved, resolveErr := resolveLLMRuntime(ctx, deps.keyProvider, cfg.userID, cfg.modelOverride, cfg.resolvePurpose)
			if resolveErr != nil {
				return nil, resolveErr
			}
			runtime = resolved
		}
		attemptRuntime = runtime
		resp, callErr := cfg.call(runtime)
		if callErr != nil {
			if !canUseLLMFallback(runtime.Model, cfg.fallbackModel, callErr) {
				return nil, callErr
			}
			primaryFailure := &llmPrimaryFailure{Model: runtime.Model, Message: callErr.Error()}
			log.Printf("process-item %s fallback item_id=%s attempt=%d primary_model=%s fallback_model=%s err=%v", cfg.purpose, ptrStringValue(cfg.itemID), cfg.attempt+1, ptrStringValue(runtime.Model), ptrStringValue(cfg.fallbackModel), callErr)
			fallbackRuntime, resolveErr := resolveLLMRuntime(ctx, deps.keyProvider, cfg.userID, cfg.fallbackModel, cfg.resolvePurpose)
			if resolveErr != nil {
				return nil, resolveErr
			}
			attemptRuntime = fallbackRuntime
			resp, callErr = cfg.call(fallbackRuntime)
			if callErr != nil {
				return nil, callErr
			}
			if resp == nil {
				return nil, fmt.Errorf("%s fallback returned nil response", cfg.purpose)
			}
			recordLLMUsage(ctx, deps.llmUsageRepo, cfg.purpose, cfg.getLLM(resp), cfg.userID, cfg.sourceID, cfg.itemID, nil, nil)
			recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, cfg.purpose, primaryFailure.Model, cfg.attempt, cfg.userID, cfg.sourceID, cfg.itemID, nil, nil, fmt.Errorf("%s", primaryFailure.Message))
			return resp, nil
		}
		if resp == nil {
			return nil, fmt.Errorf("%s returned nil response", cfg.purpose)
		}
		recordLLMUsage(ctx, deps.llmUsageRepo, cfg.purpose, cfg.getLLM(resp), cfg.userID, cfg.sourceID, cfg.itemID, nil, nil)
		return resp, nil
	})
	if err != nil {
		failedModel := executionFailedModel(attemptRuntime, cfg.modelOverride)
		if failedModel == nil && cfg.defaultRuntime != nil {
			failedModel = cfg.defaultRuntime.Model
		}
		recordLLMExecutionFailure(ctx, deps.llmExecutionRepo, cfg.purpose, failedModel, cfg.attempt, cfg.userID, cfg.sourceID, cfg.itemID, nil, nil, err)
		if cfg.onExecutionError != nil {
			return cfg.onExecutionError(err), false, nil
		}
		return nil, false, err
	}
	recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, cfg.purpose, cfg.getLLM(result), cfg.attempt, cfg.userID, cfg.sourceID, cfg.itemID, nil, nil)
	return result, strings.EqualFold(strings.TrimSpace(cfg.getVerdict(result)), "fail"), nil
}
