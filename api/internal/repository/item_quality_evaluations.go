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
	Dimensions                                                                             any
	AggregateScore, MinimumScore, MinimumConfidence, QualityThreshold, ConfidenceThreshold float64
	EscalationReason, ReasonDetail                                                         *string
	InputTokens, OutputTokens                                                              int
	EstimatedCostUSD                                                                       float64
	LatencyMS                                                                              int64
}

func (r *ItemQualityEvaluationRepo) Upsert(ctx context.Context, in ItemQualityEvaluationInput) error {
	if in.Dimensions == nil {
		in.Dimensions = map[string]any{}
	}
	dimensions, err := json.Marshal(in.Dimensions)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO item_quality_evaluations (
			item_id, kind, attempt_index, provider, requested_model, model, dimensions_json,
			aggregate_score, minimum_score, minimum_confidence, quality_threshold, confidence_threshold,
			gate_policy_version, decision, escalation_reason, reason_detail, input_tokens, output_tokens,
			estimated_cost_usd, latency_ms
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		ON CONFLICT (item_id, kind, attempt_index) DO UPDATE SET
			provider=EXCLUDED.provider, requested_model=EXCLUDED.requested_model, model=EXCLUDED.model,
			dimensions_json=EXCLUDED.dimensions_json, aggregate_score=EXCLUDED.aggregate_score,
			minimum_score=EXCLUDED.minimum_score, minimum_confidence=EXCLUDED.minimum_confidence,
			quality_threshold=EXCLUDED.quality_threshold, confidence_threshold=EXCLUDED.confidence_threshold,
			gate_policy_version=EXCLUDED.gate_policy_version, decision=EXCLUDED.decision,
			escalation_reason=EXCLUDED.escalation_reason, reason_detail=EXCLUDED.reason_detail,
			input_tokens=EXCLUDED.input_tokens, output_tokens=EXCLUDED.output_tokens,
			estimated_cost_usd=EXCLUDED.estimated_cost_usd, latency_ms=EXCLUDED.latency_ms, updated_at=NOW()`,
		in.ItemID, in.Kind, in.AttemptIndex, in.Provider, in.RequestedModel, in.Model, dimensions,
		in.AggregateScore, in.MinimumScore, in.MinimumConfidence, in.QualityThreshold, in.ConfidenceThreshold,
		in.GatePolicyVersion, in.Decision, in.EscalationReason, in.ReasonDetail, in.InputTokens, in.OutputTokens,
		in.EstimatedCostUSD, in.LatencyMS)
	return err
}

func (r *ItemQualityEvaluationRepo) LoadLatestByKind(ctx context.Context, itemID, kind string) (*model.ItemQualityEvaluation, error) {
	var out model.ItemQualityEvaluation
	var dimensions []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, item_id, kind, attempt_index, provider, requested_model, model, dimensions_json,
		       aggregate_score, minimum_score, minimum_confidence, quality_threshold, confidence_threshold,
		       gate_policy_version, decision, escalation_reason, reason_detail, input_tokens, output_tokens,
		       estimated_cost_usd, latency_ms, created_at, updated_at
		FROM item_quality_evaluations WHERE item_id=$1 AND kind=$2
		ORDER BY attempt_index DESC, created_at DESC LIMIT 1`, itemID, kind).Scan(
		&out.ID, &out.ItemID, &out.Kind, &out.AttemptIndex, &out.Provider, &out.RequestedModel, &out.Model, &dimensions,
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
	return &out, nil
}
