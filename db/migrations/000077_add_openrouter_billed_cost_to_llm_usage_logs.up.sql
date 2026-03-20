ALTER TABLE llm_usage_logs
  ADD COLUMN IF NOT EXISTS openrouter_cost_usd DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS openrouter_generation_id TEXT;

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_openrouter_generation_id
  ON llm_usage_logs (openrouter_generation_id)
  WHERE openrouter_generation_id IS NOT NULL;
