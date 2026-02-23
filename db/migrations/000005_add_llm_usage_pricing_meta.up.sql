ALTER TABLE llm_usage_logs
  ADD COLUMN IF NOT EXISTS pricing_model_family TEXT,
  ADD COLUMN IF NOT EXISTS pricing_source TEXT NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_pricing_family
  ON llm_usage_logs (pricing_model_family);
