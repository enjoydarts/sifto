package inngest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/jackc/pgx/v5/pgxpool"
)

type d1ShadowAttemptResult struct {
	Result     *jevPrecheckStepResult `json:"result"`
	RetryDelay time.Duration          `json:"retry_delay"`
}

var (
	errD1KeyNotConfigured = errors.New("D1 API key is not configured")
	errD1KeyLookup        = errors.New("D1 API key lookup failed")
)

func runD1ShadowChecks(ctx context.Context, config jevPrecheckConfig, key string, sleep func(context.Context, string, time.Duration)) (*jevPrecheckStepResult, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := step.Run(ctx, fmt.Sprintf("%s-request-%d", config.StepName, attempt+1), func(callCtx context.Context) (*d1ShadowAttemptResult, error) {
			evaluation, evalErr := config.Evaluate(callCtx, key)
			if errors.Is(evalErr, errD1KeyNotConfigured) {
				return &d1ShadowAttemptResult{Result: &jevPrecheckStepResult{Skipped: true}}, nil
			}
			if evalErr != nil {
				reason := classifyJevEscalation(evalErr)
				if errors.Is(evalErr, errD1KeyLookup) {
					reason = service.JevEscalationConfigurationError
				}
				return &d1ShadowAttemptResult{Result: jevErrorStepResult(reason, evalErr), RetryDelay: d1RetryDelay(evalErr, attempt)}, nil
			}
			if evaluation == nil {
				return &d1ShadowAttemptResult{Result: jevErrorStepResult(service.JevEscalationSchemaError, errors.New("D1 returned no evaluation"))}, nil
			}
			gate := service.EvaluateJevGate(evaluation.Dimensions, evaluation.Signals, config.Critical, config.Catalog.GatePolicy)
			return &d1ShadowAttemptResult{Result: &jevPrecheckStepResult{Evaluation: evaluation, Gate: gate, EscalationReason: string(gate.EscalationReason)}}, nil
		})
		if err != nil {
			return nil, err
		}
		if result.RetryDelay == 0 || attempt == 2 {
			return result.Result, nil
		}
		log.Printf("D1 retry item_id=%s kind=%s request=%d delay=%s reason=%s", ptrStringValue(config.ItemID), config.Kind, attempt+1, result.RetryDelay, result.Result.EscalationReason)
		sleep(ctx, fmt.Sprintf("%s-cooldown-%d", config.StepName, attempt+1), result.RetryDelay)
	}
	return nil, errors.New("D1 retry limit reached")
}

func d1RetryDelay(err error, attempt int) time.Duration {
	if errors.Is(err, context.Canceled) {
		return 0
	}
	delay := time.Duration(attempt+1) * time.Minute
	var httpErr *service.JevHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode != 429 && (httpErr.StatusCode < 500 || httpErr.StatusCode > 599) {
			return 0
		}
		if httpErr.StatusCode == 429 && delay < 2*time.Minute {
			delay = 2 * time.Minute
		}
		if httpErr.RetryAfter > delay {
			delay = httpErr.RetryAfter
		}
		return delay
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) {
		return delay
	}
	return 0
}

type d1ShadowKeyProvider interface {
	GetAPIKey(context.Context, string, string) (*string, error)
}

func enqueueD1Shadow(ctx context.Context, keys d1ShadowKeyProvider, data service.D1ShadowEventData, send func(context.Context, service.D1ShadowEventData) error) (bool, error) {
	key, err := keys.GetAPIKey(ctx, data.UserID, "d1")
	if err != nil {
		return false, err
	}
	if key == nil || strings.TrimSpace(*key) == "" {
		return false, nil
	}
	if err := send(ctx, data); err != nil {
		return false, err
	}
	return true, nil
}

func dispatchD1Shadow(ctx context.Context, deps processItemDeps, data service.D1ShadowEventData) {
	if deps.d1 == nil || deps.keyProvider == nil || deps.publisher == nil || strings.TrimSpace(data.UserID) == "" {
		return
	}
	_, err := step.Run(ctx, jevStepName("enqueue-d1-"+data.Kind, deps.d1Catalog.GatePolicy.Version, data.Attempt), func(callCtx context.Context) (bool, error) {
		return enqueueD1Shadow(callCtx, deps.keyProvider, data, deps.publisher.SendD1ShadowE)
	})
	if err != nil {
		log.Printf("enqueue D1 shadow failed open item_id=%s kind=%s err=%v", data.ItemID, data.Kind, err)
	}
}

func d1ShadowFn(client inngestgo.Client, db *pgxpool.Pool, keys *service.UserKeyProvider) (inngestgo.ServableFunction, error) {
	catalog, err := service.LoadD1Catalog()
	if err != nil {
		return nil, err
	}
	deps := processItemDeps{d1: service.NewD1ClientFromCatalog(catalog), d1Catalog: catalog, keyProvider: keys,
		qualityRepo: repository.NewItemQualityEvaluationRepo(db), llmUsageRepo: repository.NewLLMUsageLogRepo(db), llmExecutionRepo: repository.NewLLMExecutionEventRepo(db)}
	return inngestgo.CreateFunction(client, inngestgo.FunctionOpts{
		ID: "evaluate-d1-shadow", Name: "Evaluate D1 Shadow",
		Concurrency: []inngestgo.ConfigStepConcurrency{{Limit: 1}},
	}, inngestgo.EventTrigger("item/d1-shadow.requested", nil), func(ctx context.Context, input inngestgo.Input[service.D1ShadowEventData]) (any, error) {
		data := input.Event.Data
		if data.ItemID == "" || data.UserID == "" || (data.Kind != "facts" && data.Kind != "faithfulness") {
			return map[string]any{"skipped": true}, nil
		}
		ctx = withLLMExecutionTrigger(ctx, data.TriggerID, data.Reason)
		// Resolve the key on every request without putting secrets in step results or events.
		config := jevPrecheckConfig{Provider: "d1", Catalog: catalog, Client: deps.d1,
			StepName: jevStepName("evaluate-d1-"+data.Kind, catalog.GatePolicy.Version, data.Attempt), Kind: data.Kind,
			Purpose: "facts_check_precheck", Attempt: data.Attempt, UserID: &data.UserID, SourceID: &data.SourceID, ItemID: &data.ItemID,
			Critical: []string{"source_support", "contradiction_free"}}
		config.TriggerID = ptrStringOrNil(&data.TriggerID)
		if data.Kind == "faithfulness" {
			config.Purpose = "faithfulness_check_precheck"
			config.Critical = []string{"facts_support", "contradiction_free"}
		}
		config.Evaluate = func(callCtx context.Context, _ string) (*service.JevEvaluation, error) {
			key, err := keys.GetAPIKey(callCtx, data.UserID, "d1")
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errD1KeyLookup, err)
			}
			if key == nil || strings.TrimSpace(*key) == "" {
				return nil, errD1KeyNotConfigured
			}
			if data.Kind == "facts" {
				return deps.d1.EvaluateFacts(callCtx, *key, data.Title, data.Content, data.Facts)
			}
			return deps.d1.EvaluateFaithfulness(callCtx, *key, data.Title, data.Facts, data.Summary)
		}
		result, err := runD1ShadowChecks(ctx, config, "", step.Sleep)
		if err != nil {
			return nil, err
		}
		if result.Skipped {
			return result, nil
		}
		_, err = step.Run(ctx, "persist-d1-evaluation", func(callCtx context.Context) (bool, error) {
			return true, persistJevPrecheck(callCtx, deps, config, result)
		})
		return result, err
	})
}
