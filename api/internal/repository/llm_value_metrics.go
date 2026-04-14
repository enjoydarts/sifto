package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LLMValueMetricsRepo struct{ db *pgxpool.Pool }

func NewLLMValueMetricsRepo(db *pgxpool.Pool) *LLMValueMetricsRepo {
	return &LLMValueMetricsRepo{db: db}
}

type LLMValueMetricAggregate struct {
	WindowStart   time.Time
	WindowEnd     time.Time
	MonthJST      string
	Purpose       string
	Provider      string
	Model         string
	PricingSource string
	Calls         int
	TotalCostUSD  float64
	ItemCount     int
	ReadCount     int
	FavoriteCount int
	InsightCount  int
}

type LLMValueMetricSnapshot struct {
	WindowStart       string   `json:"window_start"`
	WindowEnd         string   `json:"window_end"`
	MonthJST          string   `json:"month_jst"`
	Purpose           string   `json:"purpose"`
	Provider          string   `json:"provider"`
	Model             string   `json:"model"`
	PricingSource     string   `json:"pricing_source"`
	Calls             int      `json:"calls"`
	TotalCostUSD      float64  `json:"total_cost_usd"`
	ItemCount         int      `json:"item_count"`
	ReadCount         int      `json:"read_count"`
	FavoriteCount     int      `json:"favorite_count"`
	InsightCount      int      `json:"insight_count"`
	CostToReadUSD     *float64 `json:"cost_to_read_usd,omitempty"`
	CostToFavoriteUSD *float64 `json:"cost_to_favorite_usd,omitempty"`
	CostToInsightUSD  *float64 `json:"cost_to_insight_usd,omitempty"`
	LowEfficiencyFlag bool     `json:"low_efficiency_flag"`
	AdvisoryCode      string   `json:"advisory_code"`
	AdvisoryReason    *string  `json:"advisory_reason,omitempty"`
	BenchmarkProvider *string  `json:"benchmark_provider,omitempty"`
	BenchmarkModel    *string  `json:"benchmark_model,omitempty"`
	BenchmarkMetric   *string  `json:"benchmark_metric,omitempty"`
}

func currentLLMValueMetricsWindow(now time.Time) (time.Time, time.Time) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
	}
	jst := now.In(loc)
	start := time.Date(jst.Year(), jst.Month(), 1, 0, 0, 0, 0, loc)
	end := time.Date(jst.Year(), jst.Month(), jst.Day(), 0, 0, 0, 0, loc)
	return start, end
}

func (r *LLMValueMetricsRepo) CollectCurrentMonth(ctx context.Context, userID string) ([]LLMValueMetricAggregate, error) {
	return r.CollectByMonth(ctx, userID, time.Now())
}

func (r *LLMValueMetricsRepo) CollectByMonth(ctx context.Context, userID string, month time.Time) ([]LLMValueMetricAggregate, error) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
	}
	monthJST := month.In(loc)
	monthStart := time.Date(monthJST.Year(), monthJST.Month(), 1, 0, 0, 0, 0, loc)
	nextMonthStart := monthStart.AddDate(0, 1, 0)
	windowEnd := nextMonthStart
	nowJST := time.Now().In(loc)
	if nowJST.Before(nextMonthStart) {
		windowEnd = time.Date(nowJST.Year(), nowJST.Month(), nowJST.Day(), 0, 0, 0, 0, loc)
	}
	monthKey := monthStart.Format("2006-01")
	rows, err := r.db.Query(ctx, `
		WITH usage AS (
			SELECT
				l.purpose,
				l.provider,
				l.model,
				CASE
					WHEN COUNT(DISTINCT l.pricing_source) = 1 THEN MIN(l.pricing_source)
					ELSE 'mixed(' || COUNT(DISTINCT l.pricing_source)::text || ')'
				END AS pricing_source,
				COUNT(*)::int AS calls,
				COALESCE(SUM(l.estimated_cost_usd), 0)::double precision AS total_cost_usd
			FROM llm_usage_logs l
			WHERE l.user_id::text = $1
			  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= $2
			  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < $3
			GROUP BY l.purpose, l.provider, l.model
		),
		latest_item_usage AS (
			SELECT DISTINCT ON (l.item_id, l.purpose, l.provider, l.model)
				l.item_id,
				l.purpose,
				l.provider,
				l.model,
				l.created_at
			FROM llm_usage_logs l
			WHERE l.user_id::text = $1
			  AND l.item_id IS NOT NULL
			  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') >= $2
			  AND (l.created_at AT TIME ZONE 'Asia/Tokyo') < $3
			ORDER BY l.item_id, l.purpose, l.provider, l.model, l.created_at DESC
		),
		item_actions AS (
			SELECT
				u.purpose,
				u.provider,
				u.model,
				COUNT(DISTINCT u.item_id)::int AS item_count,
				COUNT(DISTINCT CASE WHEN ir.item_id IS NOT NULL THEN u.item_id END)::int AS read_count,
				COUNT(DISTINCT CASE WHEN fb.item_id IS NOT NULL THEN u.item_id END)::int AS favorite_count,
				COUNT(DISTINCT CASE WHEN ai.id IS NOT NULL THEN u.item_id END)::int AS insight_count
			FROM latest_item_usage u
			LEFT JOIN item_reads ir
				ON ir.user_id::text = $1
				AND ir.item_id = u.item_id
				AND ir.read_at >= u.created_at
				AND (ir.read_at AT TIME ZONE 'Asia/Tokyo') >= $2
				AND (ir.read_at AT TIME ZONE 'Asia/Tokyo') < $3
			LEFT JOIN item_feedbacks fb
				ON fb.user_id::text = $1
				AND fb.item_id = u.item_id
				AND fb.is_favorite = TRUE
				AND fb.updated_at >= u.created_at
				AND (fb.updated_at AT TIME ZONE 'Asia/Tokyo') >= $2
				AND (fb.updated_at AT TIME ZONE 'Asia/Tokyo') < $3
			LEFT JOIN ask_insight_items aii
				ON aii.item_id = u.item_id
			LEFT JOIN ask_insights ai
				ON ai.id = aii.insight_id
				AND ai.user_id = $1
				AND ai.created_at >= u.created_at
				AND (ai.created_at AT TIME ZONE 'Asia/Tokyo') >= $2
				AND (ai.created_at AT TIME ZONE 'Asia/Tokyo') < $3
			GROUP BY u.purpose, u.provider, u.model
		)
		SELECT
			$2::date,
			$4::date,
			$5,
			u.purpose,
			u.provider,
			u.model,
			u.pricing_source,
			u.calls,
			u.total_cost_usd,
			COALESCE(a.item_count, 0),
			COALESCE(a.read_count, 0),
			COALESCE(a.favorite_count, 0),
			COALESCE(a.insight_count, 0)
		FROM usage u
		LEFT JOIN item_actions a
			ON a.purpose = u.purpose
			AND a.provider = u.provider
			AND a.model = u.model
		ORDER BY u.total_cost_usd DESC, u.calls DESC, u.provider ASC, u.model ASC`, userID, monthStart, nextMonthStart, windowEnd, monthKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMValueMetricAggregate
	for rows.Next() {
		var v LLMValueMetricAggregate
		if err := rows.Scan(
			&v.WindowStart,
			&v.WindowEnd,
			&v.MonthJST,
			&v.Purpose,
			&v.Provider,
			&v.Model,
			&v.PricingSource,
			&v.Calls,
			&v.TotalCostUSD,
			&v.ItemCount,
			&v.ReadCount,
			&v.FavoriteCount,
			&v.InsightCount,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *LLMValueMetricsRepo) ReplaceCurrentMonth(ctx context.Context, userID string, rows []LLMValueMetricSnapshot) error {
	windowStart, _ := currentLLMValueMetricsWindow(time.Now())
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		DELETE FROM llm_value_metrics
		WHERE user_id = $1
		  AND window_start = $2::date`, userID, windowStart.Format("2006-01-02")); err != nil {
		return err
	}

	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO llm_value_metrics (
				user_id, window_start, window_end, purpose, provider, model, metrics
			) VALUES ($1, $2::date, $3::date, $4, $5, $6, $7::jsonb)`,
			userID,
			row.WindowStart,
			row.WindowEnd,
			row.Purpose,
			row.Provider,
			row.Model,
			string(payload),
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *LLMValueMetricsRepo) ListCurrentMonth(ctx context.Context, userID string) ([]LLMValueMetricSnapshot, error) {
	windowStart, _ := currentLLMValueMetricsWindow(time.Now())
	rows, err := r.db.Query(ctx, `
		SELECT metrics
		FROM llm_value_metrics
		WHERE user_id = $1
		  AND window_start = $2::date
		ORDER BY created_at DESC, provider ASC, model ASC`, userID, windowStart.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LLMValueMetricSnapshot
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var row LLMValueMetricSnapshot
		if err := json.Unmarshal(payload, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
