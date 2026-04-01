package inngest

import (
	"context"
	"fmt"
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

	result, err := step.Run(ctx, stepName, func(ctx context.Context) (*T, error) {
		runtime := cfg.defaultRuntime
		if chooseModelOverride(cfg.modelOverride, nil) != nil {
			resolved, resolveErr := resolveLLMRuntime(ctx, deps.userSettingsRepo, deps.secretCipher, cfg.userID, cfg.modelOverride, cfg.resolvePurpose)
			if resolveErr != nil {
				return nil, resolveErr
			}
			runtime = resolved
		}
		resp, callErr := cfg.call(runtime)
		if callErr != nil {
			return nil, callErr
		}
		if resp == nil {
			return nil, fmt.Errorf("%s returned nil response", cfg.purpose)
		}
		recordLLMUsage(ctx, deps.llmUsageRepo, cfg.purpose, cfg.getLLM(resp), cfg.userID, cfg.sourceID, cfg.itemID, nil, nil)
		return resp, nil
	})
	if err != nil {
		failedModel := cfg.modelOverride
		if chooseModelOverride(failedModel, nil) == nil && cfg.defaultRuntime != nil {
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
