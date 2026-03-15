CREATE TABLE budget_forecast_alert_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  month_jst DATE NOT NULL,
  budget_usd DOUBLE PRECISION NOT NULL,
  used_cost_usd DOUBLE PRECISION NOT NULL,
  forecast_cost_usd DOUBLE PRECISION NOT NULL,
  forecast_delta_usd DOUBLE PRECISION NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, month_jst)
);

CREATE INDEX idx_budget_forecast_alert_logs_user_month
  ON budget_forecast_alert_logs (user_id, month_jst DESC);
