CREATE TABLE user_settings (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  anthropic_api_key_enc TEXT,
  anthropic_api_key_last4 TEXT,
  monthly_budget_usd DOUBLE PRECISION,
  budget_alert_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  budget_alert_threshold_pct INTEGER NOT NULL DEFAULT 20
    CHECK (budget_alert_threshold_pct >= 1 AND budget_alert_threshold_pct <= 99),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE budget_alert_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  month_jst DATE NOT NULL,
  threshold_pct INTEGER NOT NULL,
  budget_usd DOUBLE PRECISION NOT NULL,
  used_cost_usd DOUBLE PRECISION NOT NULL,
  remaining_ratio DOUBLE PRECISION NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, month_jst, threshold_pct)
);

CREATE INDEX idx_budget_alert_logs_user_month ON budget_alert_logs (user_id, month_jst DESC);
