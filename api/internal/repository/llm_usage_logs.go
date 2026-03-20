package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LLMUsageLogRepo struct{ db *pgxpool.Pool }

func NewLLMUsageLogRepo(db *pgxpool.Pool) *LLMUsageLogRepo { return &LLMUsageLogRepo{db: db} }

type LLMUsageLogInput struct {
	IdempotencyKey           *string
	UserID                   *string
	SourceID                 *string
	ItemID                   *string
	DigestID                 *string
	Provider                 string
	Model                    string
	RequestedModel           string
	ResolvedModel            string
	PricingModelFamily       string
	PricingSource            string
	Purpose                  string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	EstimatedCostUSD         float64
}

type LLMUsageLog struct {
	ID                       string    `json:"id"`
	UserID                   *string   `json:"user_id,omitempty"`
	SourceID                 *string   `json:"source_id,omitempty"`
	ItemID                   *string   `json:"item_id,omitempty"`
	DigestID                 *string   `json:"digest_id,omitempty"`
	Provider                 string    `json:"provider"`
	Model                    string    `json:"model"`
	RequestedModel           *string   `json:"requested_model,omitempty"`
	ResolvedModel            *string   `json:"resolved_model,omitempty"`
	PricingModelFamily       *string   `json:"pricing_model_family,omitempty"`
	PricingSource            string    `json:"pricing_source"`
	Purpose                  string    `json:"purpose"`
	InputTokens              int       `json:"input_tokens"`
	OutputTokens             int       `json:"output_tokens"`
	CacheCreationInputTokens int       `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int       `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64   `json:"estimated_cost_usd"`
	CreatedAt                time.Time `json:"created_at"`
}

type LLMUsageDailySummary struct {
	DateJST                  string  `json:"date_jst"`
	Provider                 string  `json:"provider"`
	Purpose                  string  `json:"purpose"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsageModelSummary struct {
	Provider                 string  `json:"provider"`
	Model                    string  `json:"model"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsageProviderMonthSummary struct {
	MonthJST                 string  `json:"month_jst"`
	Provider                 string  `json:"provider"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsagePurposeMonthSummary struct {
	MonthJST                 string  `json:"month_jst"`
	Purpose                  string  `json:"purpose"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

type LLMUsageAnalysisSummary struct {
	Provider                 string  `json:"provider"`
	Model                    string  `json:"model"`
	Purpose                  string  `json:"purpose"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

func nullIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (r *LLMUsageLogRepo) Insert(ctx context.Context, in LLMUsageLogInput) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO llm_usage_logs (
			idempotency_key, user_id, source_id, item_id, digest_id,
			provider, model, requested_model, resolved_model, pricing_model_family, pricing_source, purpose,
			input_tokens, output_tokens,
			cache_creation_input_tokens, cache_read_input_tokens,
			estimated_cost_usd
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (idempotency_key) DO NOTHING
	`,
		in.IdempotencyKey, in.UserID, in.SourceID, in.ItemID, in.DigestID,
		in.Provider, in.Model, nullIfEmpty(in.RequestedModel), nullIfEmpty(in.ResolvedModel), in.PricingModelFamily, in.PricingSource, in.Purpose,
		in.InputTokens, in.OutputTokens,
		in.CacheCreationInputTokens, in.CacheReadInputTokens,
		in.EstimatedCostUSD,
	)
	return err
}

func (r *LLMUsageLogRepo) ListByUser(ctx context.Context, userID string, limit int) ([]LLMUsageLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, source_id, item_id, digest_id,
		       provider, model, requested_model, resolved_model, pricing_model_family, pricing_source, purpose,
		       input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens,
		       estimated_cost_usd, created_at
		FROM llm_usage_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsageLog
	for rows.Next() {
		var v LLMUsageLog
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.SourceID, &v.ItemID, &v.DigestID,
			&v.Provider, &v.Model, &v.RequestedModel, &v.ResolvedModel, &v.PricingModelFamily, &v.PricingSource, &v.Purpose,
			&v.InputTokens, &v.OutputTokens, &v.CacheCreationInputTokens, &v.CacheReadInputTokens,
			&v.EstimatedCostUSD, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *LLMUsageLogRepo) DailySummaryByUser(ctx context.Context, userID string, days int) ([]LLMUsageDailySummary, error) {
	if days <= 0 || days > 365 {
		days = 14
	}
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				date_trunc('day', NOW() AT TIME ZONE 'Asia/Tokyo') - (($2::int - 1) * INTERVAL '1 day') AS since_jst,
				date_trunc('day', NOW() AT TIME ZONE 'Asia/Tokyo') + INTERVAL '1 day' AS until_jst
		)
		SELECT (l.created_at AT TIME ZONE 'Asia/Tokyo')::date::text AS date_jst,
		       l.provider,
		       l.purpose,
		       l.pricing_source,
		       COUNT(*)::int AS calls,
		       COALESCE(SUM(l.input_tokens),0)::bigint AS input_tokens,
		       COALESCE(SUM(l.output_tokens),0)::bigint AS output_tokens,
		       COALESCE(SUM(l.cache_creation_input_tokens),0)::bigint AS cache_creation_input_tokens,
		       COALESCE(SUM(l.cache_read_input_tokens),0)::bigint AS cache_read_input_tokens,
		       COALESCE(SUM(l.estimated_cost_usd),0)::double precision AS estimated_cost_usd
		FROM llm_usage_logs l
		CROSS JOIN bounds b
		WHERE l.user_id = $1
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= b.since_jst
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < b.until_jst
		GROUP BY 1,2,3,4
		ORDER BY date_jst DESC, provider ASC, purpose ASC, pricing_source ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsageDailySummary
	for rows.Next() {
		var v LLMUsageDailySummary
		if err := rows.Scan(
			&v.DateJST, &v.Provider, &v.Purpose, &v.PricingSource, &v.Calls,
			&v.InputTokens, &v.OutputTokens, &v.CacheCreationInputTokens,
			&v.CacheReadInputTokens, &v.EstimatedCostUSD,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *LLMUsageLogRepo) ModelSummaryByUser(ctx context.Context, userID string, days int) ([]LLMUsageModelSummary, error) {
	if days <= 0 || days > 365 {
		days = 14
	}
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				date_trunc('day', NOW() AT TIME ZONE 'Asia/Tokyo') - (($2::int - 1) * INTERVAL '1 day') AS since_jst,
				date_trunc('day', NOW() AT TIME ZONE 'Asia/Tokyo') + INTERVAL '1 day' AS until_jst
		)
		SELECT l.provider,
		       l.model,
		       l.pricing_source,
		       COUNT(*)::int AS calls,
		       COALESCE(SUM(l.input_tokens),0)::bigint AS input_tokens,
		       COALESCE(SUM(l.output_tokens),0)::bigint AS output_tokens,
		       COALESCE(SUM(l.cache_creation_input_tokens),0)::bigint AS cache_creation_input_tokens,
		       COALESCE(SUM(l.cache_read_input_tokens),0)::bigint AS cache_read_input_tokens,
		       COALESCE(SUM(l.estimated_cost_usd),0)::double precision AS estimated_cost_usd
		FROM llm_usage_logs l
		CROSS JOIN bounds b
		WHERE l.user_id = $1
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= b.since_jst
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < b.until_jst
		GROUP BY l.provider, l.model, l.pricing_source
		ORDER BY estimated_cost_usd DESC, calls DESC, provider ASC, model ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsageModelSummary
	for rows.Next() {
		var v LLMUsageModelSummary
		if err := rows.Scan(
			&v.Provider, &v.Model, &v.PricingSource, &v.Calls,
			&v.InputTokens, &v.OutputTokens, &v.CacheCreationInputTokens,
			&v.CacheReadInputTokens, &v.EstimatedCostUSD,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *LLMUsageLogRepo) SumEstimatedCostByUserBetween(ctx context.Context, userID string, since, until time.Time) (float64, error) {
	var total float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(estimated_cost_usd), 0)::double precision
		FROM llm_usage_logs
		WHERE user_id = $1
		  AND created_at >= $2
		  AND created_at < $3`,
		userID, since, until,
	).Scan(&total)
	return total, err
}

func (r *LLMUsageLogRepo) ProviderSummaryCurrentMonthByUser(ctx context.Context, userID string) ([]LLMUsageProviderMonthSummary, error) {
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				date_trunc('month', NOW() AT TIME ZONE 'Asia/Tokyo') AS month_start_jst,
				date_trunc('month', NOW() AT TIME ZONE 'Asia/Tokyo') + INTERVAL '1 month' AS next_month_start_jst
		)
		SELECT TO_CHAR(b.month_start_jst, 'YYYY-MM') AS month_jst,
		       l.provider,
		       COUNT(*)::int AS calls,
		       COALESCE(SUM(l.input_tokens),0)::bigint AS input_tokens,
		       COALESCE(SUM(l.output_tokens),0)::bigint AS output_tokens,
		       COALESCE(SUM(l.cache_creation_input_tokens),0)::bigint AS cache_creation_input_tokens,
		       COALESCE(SUM(l.cache_read_input_tokens),0)::bigint AS cache_read_input_tokens,
		       COALESCE(SUM(l.estimated_cost_usd),0)::double precision AS estimated_cost_usd
		FROM llm_usage_logs l
		CROSS JOIN bounds b
		WHERE l.user_id = $1
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= b.month_start_jst
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < b.next_month_start_jst
		GROUP BY 1,2
		ORDER BY estimated_cost_usd DESC, calls DESC, provider ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsageProviderMonthSummary
	for rows.Next() {
		var v LLMUsageProviderMonthSummary
		if err := rows.Scan(
			&v.MonthJST, &v.Provider, &v.Calls,
			&v.InputTokens, &v.OutputTokens, &v.CacheCreationInputTokens,
			&v.CacheReadInputTokens, &v.EstimatedCostUSD,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *LLMUsageLogRepo) PurposeSummaryCurrentMonthByUser(ctx context.Context, userID string) ([]LLMUsagePurposeMonthSummary, error) {
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				date_trunc('month', NOW() AT TIME ZONE 'Asia/Tokyo') AS month_start_jst,
				date_trunc('month', NOW() AT TIME ZONE 'Asia/Tokyo') + INTERVAL '1 month' AS next_month_start_jst
		)
		SELECT TO_CHAR(b.month_start_jst, 'YYYY-MM') AS month_jst,
		       l.purpose,
		       COUNT(*)::int AS calls,
		       COALESCE(SUM(l.input_tokens),0)::bigint AS input_tokens,
		       COALESCE(SUM(l.output_tokens),0)::bigint AS output_tokens,
		       COALESCE(SUM(l.cache_creation_input_tokens),0)::bigint AS cache_creation_input_tokens,
		       COALESCE(SUM(l.cache_read_input_tokens),0)::bigint AS cache_read_input_tokens,
		       COALESCE(SUM(l.estimated_cost_usd),0)::double precision AS estimated_cost_usd
		FROM llm_usage_logs l
		CROSS JOIN bounds b
		WHERE l.user_id = $1
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= b.month_start_jst
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < b.next_month_start_jst
		GROUP BY 1,2
		ORDER BY estimated_cost_usd DESC, calls DESC, purpose ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsagePurposeMonthSummary
	for rows.Next() {
		var v LLMUsagePurposeMonthSummary
		if err := rows.Scan(
			&v.MonthJST, &v.Purpose, &v.Calls,
			&v.InputTokens, &v.OutputTokens, &v.CacheCreationInputTokens,
			&v.CacheReadInputTokens, &v.EstimatedCostUSD,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *LLMUsageLogRepo) AnalysisSummaryByUser(ctx context.Context, userID string, days int) ([]LLMUsageAnalysisSummary, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := r.db.Query(ctx, `
		WITH bounds AS (
			SELECT
				date_trunc('day', NOW() AT TIME ZONE 'Asia/Tokyo') - (($2::int - 1) * INTERVAL '1 day') AS since_jst,
				date_trunc('day', NOW() AT TIME ZONE 'Asia/Tokyo') + INTERVAL '1 day' AS until_jst
		)
		SELECT l.provider,
		       l.model,
		       l.purpose,
		       l.pricing_source,
		       COUNT(*)::int AS calls,
		       COALESCE(SUM(l.input_tokens),0)::bigint AS input_tokens,
		       COALESCE(SUM(l.output_tokens),0)::bigint AS output_tokens,
		       COALESCE(SUM(l.cache_creation_input_tokens),0)::bigint AS cache_creation_input_tokens,
		       COALESCE(SUM(l.cache_read_input_tokens),0)::bigint AS cache_read_input_tokens,
		       COALESCE(SUM(l.estimated_cost_usd),0)::double precision AS estimated_cost_usd
		FROM llm_usage_logs l
		CROSS JOIN bounds b
		WHERE l.user_id = $1
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= b.since_jst
		  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < b.until_jst
		GROUP BY l.provider, l.model, l.purpose, l.pricing_source
		ORDER BY estimated_cost_usd DESC, calls DESC, provider ASC, model ASC, purpose ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsageAnalysisSummary
	for rows.Next() {
		var v LLMUsageAnalysisSummary
		if err := rows.Scan(
			&v.Provider, &v.Model, &v.Purpose, &v.PricingSource, &v.Calls,
			&v.InputTokens, &v.OutputTokens, &v.CacheCreationInputTokens,
			&v.CacheReadInputTokens, &v.EstimatedCostUSD,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
