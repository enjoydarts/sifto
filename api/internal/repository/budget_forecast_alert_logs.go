package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetForecastAlertLogRepo struct{ db *pgxpool.Pool }

func NewBudgetForecastAlertLogRepo(db *pgxpool.Pool) *BudgetForecastAlertLogRepo {
	return &BudgetForecastAlertLogRepo{db: db}
}

func (r *BudgetForecastAlertLogRepo) Exists(ctx context.Context, userID string, monthJST time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM budget_forecast_alert_logs
			WHERE user_id = $1
			  AND month_jst = $2::date
		)`, userID, monthJST.Format("2006-01-02"),
	).Scan(&exists)
	return exists, err
}

func (r *BudgetForecastAlertLogRepo) Insert(ctx context.Context, userID string, monthJST time.Time, budgetUSD, usedCostUSD, forecastCostUSD, forecastDeltaUSD float64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO budget_forecast_alert_logs (
			user_id, month_jst, budget_usd, used_cost_usd, forecast_cost_usd, forecast_delta_usd
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, month_jst) DO NOTHING`,
		userID, monthJST.Format("2006-01-02"), budgetUSD, usedCostUSD, forecastCostUSD, forecastDeltaUSD,
	)
	return err
}
