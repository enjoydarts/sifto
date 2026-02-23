ALTER TABLE llm_usage_logs
  ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_usage_logs_idempotency_key
  ON llm_usage_logs (idempotency_key)
  WHERE idempotency_key IS NOT NULL;
