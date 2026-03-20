DROP INDEX IF EXISTS idx_llm_usage_logs_openrouter_generation_id;

ALTER TABLE llm_usage_logs
  DROP COLUMN IF EXISTS openrouter_generation_id,
  DROP COLUMN IF EXISTS openrouter_cost_usd;
