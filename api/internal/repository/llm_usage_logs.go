package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LLMUsageLogRepo struct{ db *pgxpool.Pool }

func NewLLMUsageLogRepo(db *pgxpool.Pool) *LLMUsageLogRepo { return &LLMUsageLogRepo{db: db} }

type LLMUsageLogInput struct {
	IdempotencyKey          *string
	UserID                  *string
	SourceID                *string
	ItemID                  *string
	DigestID                *string
	Provider                string
	Model                   string
	PricingModelFamily      string
	PricingSource           string
	Purpose                 string
	InputTokens             int
	OutputTokens            int
	CacheCreationInputTokens int
	CacheReadInputTokens    int
	EstimatedCostUSD        float64
}

type LLMUsageLog struct {
	ID                       string     `json:"id"`
	UserID                   *string    `json:"user_id,omitempty"`
	SourceID                 *string    `json:"source_id,omitempty"`
	ItemID                   *string    `json:"item_id,omitempty"`
	DigestID                 *string    `json:"digest_id,omitempty"`
	Provider                 string     `json:"provider"`
	Model                    string     `json:"model"`
	PricingModelFamily       *string    `json:"pricing_model_family,omitempty"`
	PricingSource            string     `json:"pricing_source"`
	Purpose                  string     `json:"purpose"`
	InputTokens              int        `json:"input_tokens"`
	OutputTokens             int        `json:"output_tokens"`
	CacheCreationInputTokens int        `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int        `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64    `json:"estimated_cost_usd"`
	CreatedAt                time.Time  `json:"created_at"`
}

type LLMUsageDailySummary struct {
	DateJST                  string  `json:"date_jst"`
	Purpose                  string  `json:"purpose"`
	PricingSource            string  `json:"pricing_source"`
	Calls                    int     `json:"calls"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

func (r *LLMUsageLogRepo) Insert(ctx context.Context, in LLMUsageLogInput) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO llm_usage_logs (
			idempotency_key, user_id, source_id, item_id, digest_id,
			provider, model, pricing_model_family, pricing_source, purpose,
			input_tokens, output_tokens,
			cache_creation_input_tokens, cache_read_input_tokens,
			estimated_cost_usd
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (idempotency_key) DO NOTHING
	`,
		in.IdempotencyKey, in.UserID, in.SourceID, in.ItemID, in.DigestID,
		in.Provider, in.Model, in.PricingModelFamily, in.PricingSource, in.Purpose,
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
		       provider, model, pricing_model_family, pricing_source, purpose,
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
			&v.Provider, &v.Model, &v.PricingModelFamily, &v.PricingSource, &v.Purpose,
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
		SELECT (created_at AT TIME ZONE 'Asia/Tokyo')::date::text AS date_jst,
		       purpose,
		       pricing_source,
		       COUNT(*)::int AS calls,
		       COALESCE(SUM(input_tokens),0)::bigint AS input_tokens,
		       COALESCE(SUM(output_tokens),0)::bigint AS output_tokens,
		       COALESCE(SUM(cache_creation_input_tokens),0)::bigint AS cache_creation_input_tokens,
		       COALESCE(SUM(cache_read_input_tokens),0)::bigint AS cache_read_input_tokens,
		       COALESCE(SUM(estimated_cost_usd),0)::double precision AS estimated_cost_usd
		FROM llm_usage_logs
		WHERE user_id = $1
		  AND created_at >= (NOW() AT TIME ZONE 'UTC') - ($2::int * INTERVAL '1 day')
		GROUP BY 1,2,3
		ORDER BY date_jst DESC, purpose ASC, pricing_source ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMUsageDailySummary
	for rows.Next() {
		var v LLMUsageDailySummary
		if err := rows.Scan(
			&v.DateJST, &v.Purpose, &v.PricingSource, &v.Calls,
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
