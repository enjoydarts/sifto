package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemQualityEvaluationRepo struct{ db *pgxpool.Pool }

func NewItemQualityEvaluationRepo(db *pgxpool.Pool) *ItemQualityEvaluationRepo {
	return &ItemQualityEvaluationRepo{db: db}
}

type ItemQualityEvaluationInput struct {
	ItemID, Kind, Provider, RequestedModel, Model, GatePolicyVersion, Decision             string
	AttemptIndex                                                                           int
	Dimensions, Signals, SignalThresholds                                                  any
	AggregateScore, MinimumScore, MinimumConfidence, QualityThreshold, ConfidenceThreshold float64
	EscalationReason, ReasonDetail                                                         *string
	InputTokens, OutputTokens                                                              int
	EstimatedCostUSD                                                                       float64
	LatencyMS                                                                              int64
	CurrentTriggerID                                                                       *string
}

func (r *ItemQualityEvaluationRepo) Upsert(ctx context.Context, in ItemQualityEvaluationInput) error {
	if in.Dimensions == nil {
		in.Dimensions = map[string]any{}
	}
	if in.Signals == nil {
		in.Signals = map[string]any{}
	}
	if in.SignalThresholds == nil {
		in.SignalThresholds = map[string]any{}
	}
	dimensions, err := json.Marshal(in.Dimensions)
	if err != nil {
		return err
	}
	signals, err := json.Marshal(in.Signals)
	if err != nil {
		return err
	}
	signalThresholds, err := json.Marshal(in.SignalThresholds)
	if err != nil {
		return err
	}
	// A delayed shadow evaluation must not overwrite a gate for the same
	// candidate. Attempt indexes are reused across processing runs, so allow a
	// shadow to replace an old gate after a new candidate has been generated.
	_, err = r.db.Exec(ctx, `
		INSERT INTO item_quality_evaluations (
			item_id, kind, attempt_index, provider, requested_model, model, dimensions_json, signals_json, signal_thresholds_json,
			aggregate_score, minimum_score, minimum_confidence, quality_threshold, confidence_threshold,
			gate_policy_version, decision, escalation_reason, reason_detail, input_tokens, output_tokens,
			estimated_cost_usd, latency_ms
		) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22
		WHERE $23::text IS NULL OR $23 = COALESCE((
			SELECT e.trigger_id FROM llm_execution_events e
			WHERE e.user_id = (SELECT s.user_id FROM items i JOIN sources s ON s.id = i.source_id WHERE i.id = $1)
			  AND e.item_id = $1 AND e.purpose = 'facts' AND e.status = 'success'
			ORDER BY e.created_at DESC LIMIT 1
		), $23)
		ON CONFLICT (item_id, kind, attempt_index, provider) DO UPDATE SET
			provider=EXCLUDED.provider, requested_model=EXCLUDED.requested_model, model=EXCLUDED.model,
			dimensions_json=EXCLUDED.dimensions_json, signals_json=EXCLUDED.signals_json,
			signal_thresholds_json=EXCLUDED.signal_thresholds_json, aggregate_score=EXCLUDED.aggregate_score,
			minimum_score=EXCLUDED.minimum_score, minimum_confidence=EXCLUDED.minimum_confidence,
			quality_threshold=EXCLUDED.quality_threshold, confidence_threshold=EXCLUDED.confidence_threshold,
			gate_policy_version=EXCLUDED.gate_policy_version, decision=EXCLUDED.decision,
			escalation_reason=EXCLUDED.escalation_reason, reason_detail=EXCLUDED.reason_detail,
			input_tokens=EXCLUDED.input_tokens, output_tokens=EXCLUDED.output_tokens,
			estimated_cost_usd=EXCLUDED.estimated_cost_usd, latency_ms=EXCLUDED.latency_ms, updated_at=NOW()
		WHERE NOT (
			item_quality_evaluations.provider = 'd1'
			AND item_quality_evaluations.gate_policy_version = 'd1-conditional-gate-v1'
			AND EXCLUDED.gate_policy_version = 'd1-shadow-v1'
		) OR EXISTS (
			SELECT 1 FROM llm_execution_events e
			WHERE e.user_id = (SELECT s.user_id FROM items i JOIN sources s ON s.id = i.source_id WHERE i.id = EXCLUDED.item_id)
			  AND e.item_id = EXCLUDED.item_id AND e.status = 'success'
			  AND e.purpose = CASE EXCLUDED.kind WHEN 'facts' THEN 'facts' WHEN 'faithfulness' THEN 'summary' END
			  AND e.attempt_index = EXCLUDED.attempt_index
			  AND e.created_at > item_quality_evaluations.updated_at
		)`,
		in.ItemID, in.Kind, in.AttemptIndex, in.Provider, in.RequestedModel, in.Model, dimensions, signals, signalThresholds,
		in.AggregateScore, in.MinimumScore, in.MinimumConfidence, in.QualityThreshold, in.ConfidenceThreshold,
		in.GatePolicyVersion, in.Decision, in.EscalationReason, in.ReasonDetail, in.InputTokens, in.OutputTokens,
		in.EstimatedCostUSD, in.LatencyMS, in.CurrentTriggerID)
	return err
}

func (r *ItemQualityEvaluationRepo) LoadLatestByKind(ctx context.Context, itemID, kind string) (*model.ItemQualityEvaluation, error) {
	return r.LoadLatestByKindAndProvider(ctx, itemID, kind, "jev")
}

func (r *ItemQualityEvaluationRepo) LoadLatestByKindAndProvider(ctx context.Context, itemID, kind, provider string) (*model.ItemQualityEvaluation, error) {
	var out model.ItemQualityEvaluation
	var dimensions, signals, signalThresholds []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, item_id, kind, attempt_index, provider, requested_model, model, dimensions_json, signals_json, signal_thresholds_json,
		       aggregate_score, minimum_score, minimum_confidence, quality_threshold, confidence_threshold,
		       gate_policy_version, decision, escalation_reason, reason_detail, input_tokens, output_tokens,
		       estimated_cost_usd, latency_ms, created_at, updated_at
		FROM item_quality_evaluations WHERE item_id=$1 AND kind=$2 AND provider=$3
		ORDER BY attempt_index DESC, created_at DESC LIMIT 1`, itemID, kind, provider).Scan(
		&out.ID, &out.ItemID, &out.Kind, &out.AttemptIndex, &out.Provider, &out.RequestedModel, &out.Model, &dimensions, &signals, &signalThresholds,
		&out.AggregateScore, &out.MinimumScore, &out.MinimumConfidence, &out.QualityThreshold, &out.ConfidenceThreshold,
		&out.GatePolicyVersion, &out.Decision, &out.EscalationReason, &out.ReasonDetail, &out.InputTokens, &out.OutputTokens,
		&out.EstimatedCostUSD, &out.LatencyMS, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dimensions, &out.Dimensions); err != nil {
		return nil, err
	}
	if out.Dimensions == nil {
		out.Dimensions = map[string]model.ItemQualityDimension{}
	}
	if err := json.Unmarshal(signals, &out.Signals); err != nil {
		return nil, err
	}
	if out.Signals == nil {
		out.Signals = map[string]model.ItemQualitySignal{}
	}
	if err := json.Unmarshal(signalThresholds, &out.SignalThresholds); err != nil {
		return nil, err
	}
	if out.SignalThresholds == nil {
		out.SignalThresholds = map[string]float64{}
	}
	return &out, nil
}
