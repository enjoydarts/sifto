package inngest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/inngest/inngestgo/step"
)

type jevPrecheckStepResult struct {
	Skipped          bool                   `json:"skipped"`
	Evaluation       *service.JevEvaluation `json:"evaluation,omitempty"`
	Gate             service.JevGateResult  `json:"gate"`
	EscalationReason string                 `json:"escalation_reason,omitempty"`
	ReasonDetail     string                 `json:"reason_detail,omitempty"`
}

type jevPrecheckConfig struct {
	Provider  string
	Catalog   service.JevCatalog
	Client    *service.JevClient
	StepName  string
	Kind      string
	Purpose   string
	Attempt   int
	UserID    *string
	SourceID  *string
	ItemID    *string
	TriggerID *string
	Critical  []string
	Evaluate  func(context.Context, string) (*service.JevEvaluation, error)
	Timeout   time.Duration
}

func executeFactsQualityGate(ctx context.Context, deps processItemDeps, data processItemEventData, itemID string, userID *string, attempt int, title *string, content string, facts []string) string {
	jevConfig := jevPrecheckConfig{
		Provider: "jev", Catalog: deps.jevCatalog, Client: deps.jev,
		StepName: jevStepName("check-facts", deps.jevCatalog.GatePolicy.Version, attempt), Kind: "facts", Purpose: "facts_check_precheck",
		Attempt: attempt, UserID: userID, SourceID: &data.SourceID, ItemID: &itemID,
		Critical: []string{"source_support", "contradiction_free"},
		Evaluate: func(callCtx context.Context, key string) (*service.JevEvaluation, error) {
			return deps.jev.EvaluateFacts(callCtx, key, ptrStringValue(title), content, facts)
		},
	}
	d1Config := jevConfig
	d1Config.Provider, d1Config.Catalog, d1Config.Client = "d1", deps.d1Catalog, deps.d1
	d1Config.StepName, d1Config.TriggerID = "check-facts-d1", &data.TriggerID
	d1Config.Evaluate = func(callCtx context.Context, key string) (*service.JevEvaluation, error) {
		return deps.d1.EvaluateFacts(callCtx, key, ptrStringValue(title), content, facts)
	}
	return executeConditionalQualityGate(ctx, deps, jevConfig, d1Config, func() {
		executeD1FactsShadow(ctx, deps, data, itemID, userID, attempt, title, content, facts)
	})
}

func executeFaithfulnessQualityGate(ctx context.Context, deps processItemDeps, data processItemEventData, itemID string, userID *string, attempt int, title *string, facts []string, summary string) string {
	jevConfig := jevPrecheckConfig{
		Provider: "jev", Catalog: deps.jevCatalog, Client: deps.jev,
		StepName: jevStepName("check-summary-faithfulness", deps.jevCatalog.GatePolicy.Version, attempt), Kind: "faithfulness", Purpose: "faithfulness_check_precheck",
		Attempt: attempt, UserID: userID, SourceID: &data.SourceID, ItemID: &itemID,
		Critical: []string{"facts_support", "contradiction_free"},
		Evaluate: func(callCtx context.Context, key string) (*service.JevEvaluation, error) {
			return deps.jev.EvaluateFaithfulness(callCtx, key, ptrStringValue(title), facts, summary)
		},
	}
	d1Config := jevConfig
	d1Config.Provider, d1Config.Catalog, d1Config.Client = "d1", deps.d1Catalog, deps.d1
	d1Config.StepName, d1Config.TriggerID = "check-summary-faithfulness-d1", &data.TriggerID
	d1Config.Evaluate = func(callCtx context.Context, key string) (*service.JevEvaluation, error) {
		return deps.d1.EvaluateFaithfulness(callCtx, key, ptrStringValue(title), facts, summary)
	}
	return executeConditionalQualityGate(ctx, deps, jevConfig, d1Config, func() {
		executeD1FaithfulnessShadow(ctx, deps, data, itemID, userID, attempt, title, facts, summary)
	})
}

func executeD1FactsShadow(ctx context.Context, deps processItemDeps, data processItemEventData, itemID string, userID *string, attempt int, title *string, content string, facts []string) {
	if deps.d1 == nil {
		return
	}
	dispatchD1Shadow(ctx, deps, service.D1ShadowEventData{
		ItemID: itemID, SourceID: data.SourceID, UserID: ptrStringValue(userID), TriggerID: data.TriggerID, Reason: data.Reason,
		Kind: "facts", Attempt: attempt, Title: ptrStringValue(title), Content: content, Facts: facts,
	})
}

func executeD1FaithfulnessShadow(ctx context.Context, deps processItemDeps, data processItemEventData, itemID string, userID *string, attempt int, title *string, facts []string, summary string) {
	if deps.d1 == nil {
		return
	}
	dispatchD1Shadow(ctx, deps, service.D1ShadowEventData{
		ItemID: itemID, SourceID: data.SourceID, UserID: ptrStringValue(userID), TriggerID: data.TriggerID, Reason: data.Reason,
		Kind: "faithfulness", Attempt: attempt, Title: ptrStringValue(title), Facts: facts, Summary: summary,
	})
}

func executeJevPrecheck(ctx context.Context, deps processItemDeps, config jevPrecheckConfig) bool {
	result := executeQualityPrecheck(ctx, deps, config)
	return result != nil && !result.Skipped && result.Gate.Decision == service.JevDecisionAccepted
}

func executeQualityPrecheck(ctx context.Context, deps processItemDeps, config jevPrecheckConfig) *jevPrecheckStepResult {
	if config.Client == nil || deps.keyProvider == nil || config.UserID == nil || strings.TrimSpace(*config.UserID) == "" {
		return nil
	}
	result, err := runQualityPrecheckStep(ctx, deps.keyProvider, config)
	if err != nil {
		log.Printf("%s precheck step failed open item_id=%s kind=%s err=%v", config.Provider, ptrStringValue(config.ItemID), config.Kind, err)
		return nil
	}
	if result == nil || result.Skipped {
		return result
	}
	if err := persistJevPrecheck(ctx, deps, config, result); err != nil {
		log.Printf("persist %s evaluation item_id=%s kind=%s: %v", config.Provider, ptrStringValue(config.ItemID), config.Kind, err)
	}
	return result
}

func runQualityPrecheckStep(ctx context.Context, keys d1ShadowKeyProvider, config jevPrecheckConfig) (*jevPrecheckStepResult, error) {
	return step.Run(ctx, config.StepName, func(stepCtx context.Context) (*jevPrecheckStepResult, error) {
		if config.Timeout > 0 {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(stepCtx, config.Timeout)
			defer cancel()
		}
		key, keyErr := keys.GetAPIKey(stepCtx, *config.UserID, config.Provider)
		if keyErr != nil {
			return jevErrorStepResult(service.JevEscalationConfigurationError, keyErr), nil
		}
		if key == nil || strings.TrimSpace(*key) == "" {
			return &jevPrecheckStepResult{Skipped: true}, nil
		}
		evaluation, evalErr := config.Evaluate(stepCtx, *key)
		if evalErr != nil {
			reason := classifyJevEscalation(evalErr)
			return jevErrorStepResult(reason, evalErr), nil
		}
		if evaluation == nil {
			return jevErrorStepResult(service.JevEscalationSchemaError, fmt.Errorf("%s returned no evaluation", config.Provider)), nil
		}
		gate := service.EvaluateJevGate(evaluation.Dimensions, evaluation.Signals, config.Critical, config.Catalog.GatePolicy)
		return &jevPrecheckStepResult{Evaluation: evaluation, Gate: gate, EscalationReason: string(gate.EscalationReason)}, nil
	})
}

func jevStepName(prefix, policyVersion string, attempt int) string {
	version := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, strings.TrimSpace(policyVersion))
	if version == "" {
		version = "unversioned"
	}
	return fmt.Sprintf("%s-%s-%d", prefix, version, attempt+1)
}

func jevErrorStepResult(reason service.JevEscalationReason, err error) *jevPrecheckStepResult {
	detail := ""
	if err != nil {
		detail = strings.TrimSpace(err.Error())
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	return &jevPrecheckStepResult{Gate: service.JevGateResult{Decision: service.JevDecisionError}, EscalationReason: string(reason), ReasonDetail: detail}
}

func classifyJevEscalation(err error) service.JevEscalationReason {
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline") {
		return service.JevEscalationTimeout
	}
	if strings.Contains(err.Error(), "HTTP status=") || strings.HasPrefix(err.Error(), "Jev request:") || strings.HasPrefix(err.Error(), "D1 request:") {
		return service.JevEscalationHTTPError
	}
	return service.JevEscalationSchemaError
}

func persistJevPrecheck(ctx context.Context, deps processItemDeps, config jevPrecheckConfig, result *jevPrecheckStepResult) error {
	policy := config.Catalog.GatePolicy
	input := repository.ItemQualityEvaluationInput{
		ItemID: ptrStringValue(config.ItemID), Kind: config.Kind, AttemptIndex: config.Attempt, Provider: config.Provider,
		RequestedModel: config.Catalog.DefaultModel, Model: config.Catalog.DefaultModel,
		Dimensions: map[string]service.JevDimension{}, Signals: map[string]service.JevSignal{}, SignalThresholds: policy.SignalThresholds,
		QualityThreshold: policy.AggregateThreshold, ConfidenceThreshold: policy.MinimumConfidence,
		GatePolicyVersion: policy.Version, Decision: string(result.Gate.Decision),
		CurrentTriggerID: config.TriggerID,
	}
	if result.EscalationReason != "" {
		input.EscalationReason = &result.EscalationReason
	}
	if result.ReasonDetail != "" {
		input.ReasonDetail = &result.ReasonDetail
	}
	if evaluation := result.Evaluation; evaluation != nil {
		input.RequestedModel, input.Model, input.Dimensions, input.Signals = evaluation.RequestedModel, evaluation.Model, evaluation.Dimensions, evaluation.Signals
		input.AggregateScore, input.MinimumScore, input.MinimumConfidence = result.Gate.AggregateScore, result.Gate.MinimumScore, result.Gate.MinimumConfidence
		input.InputTokens, input.OutputTokens, input.EstimatedCostUSD, input.LatencyMS = evaluation.Usage.InputTokens, evaluation.Usage.OutputTokens, evaluation.Usage.EstimatedCostUSD, evaluation.LatencyMS
		usage := &service.LLMUsage{Provider: config.Provider, Model: evaluation.Model, RequestedModel: evaluation.RequestedModel, ResolvedModel: evaluation.Model, PricingSource: evaluation.Usage.PricingSource, InputTokens: evaluation.Usage.InputTokens, OutputTokens: evaluation.Usage.OutputTokens, EstimatedCostUSD: evaluation.Usage.EstimatedCostUSD}
		recordLLMUsage(ctx, deps.llmUsageRepo, config.Purpose, usage, config.UserID, config.SourceID, config.ItemID, nil, nil)
		recordLLMExecutionSuccess(ctx, deps.llmExecutionRepo, config.Purpose, usage, config.Attempt, config.UserID, config.SourceID, config.ItemID, nil, nil)
	}
	if deps.qualityRepo != nil {
		if err := deps.qualityRepo.Upsert(ctx, input); err != nil {
			return err
		}
		bumpProcessItemDetailCacheVersion(ctx, deps.cache, input.ItemID)
	}
	return nil
}
