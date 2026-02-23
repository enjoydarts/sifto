package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetAlertLogRepo struct{ db *pgxpool.Pool }

func NewBudgetAlertLogRepo(db *pgxpool.Pool) *BudgetAlertLogRepo { return &BudgetAlertLogRepo{db: db} }

func (r *BudgetAlertLogRepo) Exists(ctx context.Context, userID string, monthJST time.Time, thresholdPct int) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM budget_alert_logs
			WHERE user_id = $1
			  AND month_jst = $2::date
			  AND threshold_pct = $3
		)`, userID, monthJST.Format("2006-01-02"), thresholdPct,
	).Scan(&exists)
	return exists, err
}

func (r *BudgetAlertLogRepo) Insert(ctx context.Context, userID string, monthJST time.Time, thresholdPct int, budgetUSD, usedCostUSD, remainingRatio float64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO budget_alert_logs (
			user_id, month_jst, threshold_pct, budget_usd, used_cost_usd, remaining_ratio
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, month_jst, threshold_pct) DO NOTHING`,
		userID, monthJST.Format("2006-01-02"), thresholdPct, budgetUSD, usedCostUSD, remainingRatio,
	)
	return err
}
