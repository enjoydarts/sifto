package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LLMExecutionEventRepo struct{ db *pgxpool.Pool }

func NewLLMExecutionEventRepo(db *pgxpool.Pool) *LLMExecutionEventRepo {
	return &LLMExecutionEventRepo{db: db}
}

type LLMExecutionEventInput struct {
	IdempotencyKey *string
	UserID         *string
	SourceID       *string
	ItemID         *string
	DigestID       *string
	TriggerID      *string
	TriggerReason  *string
	Provider       string
	Model          string
	Purpose        string
	Status         string
	AttemptIndex   int
	EmptyResponse  bool
	ErrorKind      *string
	ErrorMessage   *string
}

type LLMExecutionCurrentMonthSummary struct {
	MonthJST         string  `json:"month_jst"`
	Purpose          string  `json:"purpose"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Attempts         int     `json:"attempts"`
	Successes        int     `json:"successes"`
	Failures         int     `json:"failures"`
	Retries          int     `json:"retries"`
	EmptyResponses   int     `json:"empty_responses"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	FailureRatePct   float64 `json:"failure_rate_pct"`
	RetryRatePct     float64 `json:"retry_rate_pct"`
	EmptyRatePct     float64 `json:"empty_rate_pct"`
}

func (r *LLMExecutionEventRepo) Insert(ctx context.Context, in LLMExecutionEventInput) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO llm_execution_events (
			idempotency_key,
			user_id, source_id, item_id, digest_id,
			trigger_id, trigger_reason,
			provider, model, purpose, status, attempt_index,
			empty_response, error_kind, error_message
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (idempotency_key) DO NOTHING
	`,
		in.IdempotencyKey,
		in.UserID, in.SourceID, in.ItemID, in.DigestID,
		in.TriggerID, in.TriggerReason,
		in.Provider, in.Model, in.Purpose, in.Status, in.AttemptIndex,
		in.EmptyResponse, in.ErrorKind, in.ErrorMessage,
	)
	return err
}

func (r *LLMExecutionEventRepo) CurrentMonthSummaryByUser(ctx context.Context, userID string) ([]LLMExecutionCurrentMonthSummary, error) {
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				date_trunc('month', NOW() AT TIME ZONE 'Asia/Tokyo') AS month_start_jst,
				date_trunc('month', NOW() AT TIME ZONE 'Asia/Tokyo') + INTERVAL '1 month' AS next_month_start_jst
		),
		usage_costs AS (
			SELECT
				l.user_id,
				l.purpose,
				l.provider,
				l.model,
				COALESCE(SUM(l.estimated_cost_usd), 0)::double precision AS estimated_cost_usd
			FROM llm_usage_logs l
			CROSS JOIN bounds b
			WHERE l.user_id = $1
			  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= b.month_start_jst
			  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < b.next_month_start_jst
			GROUP BY l.user_id, l.purpose, l.provider, l.model
		)
		SELECT TO_CHAR(b.month_start_jst, 'YYYY-MM') AS month_jst,
		       e.purpose,
		       e.provider,
		       e.model,
		       COUNT(*)::int AS attempts,
		       COUNT(*) FILTER (WHERE e.status = 'success')::int AS successes,
		       COUNT(*) FILTER (WHERE e.status = 'failure')::int AS failures,
		       COUNT(*) FILTER (WHERE e.attempt_index > 0)::int AS retries,
		       COUNT(*) FILTER (WHERE e.empty_response)::int AS empty_responses,
		       COALESCE(MAX(u.estimated_cost_usd), 0)::double precision AS estimated_cost_usd,
		       CASE WHEN COUNT(*) = 0 THEN 0 ELSE ROUND((COUNT(*) FILTER (WHERE e.status = 'failure')::numeric * 100.0) / COUNT(*), 1) END::double precision AS failure_rate_pct,
		       CASE WHEN COUNT(*) = 0 THEN 0 ELSE ROUND((COUNT(*) FILTER (WHERE e.attempt_index > 0)::numeric * 100.0) / COUNT(*), 1) END::double precision AS retry_rate_pct,
		       CASE WHEN COUNT(*) = 0 THEN 0 ELSE ROUND((COUNT(*) FILTER (WHERE e.empty_response)::numeric * 100.0) / COUNT(*), 1) END::double precision AS empty_rate_pct
		FROM llm_execution_events e
		CROSS JOIN bounds b
		LEFT JOIN usage_costs u
		  ON u.user_id = e.user_id
		 AND u.purpose = e.purpose
		 AND u.provider = e.provider
		 AND u.model = e.model
		WHERE e.user_id = $1
		  AND (e.created_at AT TIME ZONE 'Asia/Tokyo') >= b.month_start_jst
		  AND (e.created_at AT TIME ZONE 'Asia/Tokyo') < b.next_month_start_jst
		GROUP BY 1,2,3,4
		ORDER BY estimated_cost_usd DESC, failures DESC, retries DESC, attempts DESC, purpose ASC, provider ASC, model ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LLMExecutionCurrentMonthSummary, 0)
	for rows.Next() {
		var v LLMExecutionCurrentMonthSummary
		if err := rows.Scan(
			&v.MonthJST, &v.Purpose, &v.Provider, &v.Model,
			&v.Attempts, &v.Successes, &v.Failures, &v.Retries, &v.EmptyResponses,
			&v.EstimatedCostUSD,
			&v.FailureRatePct, &v.RetryRatePct, &v.EmptyRatePct,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
