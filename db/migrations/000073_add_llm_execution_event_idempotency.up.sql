ALTER TABLE llm_execution_events
  ADD COLUMN IF NOT EXISTS idempotency_key text,
  ADD COLUMN IF NOT EXISTS trigger_id text,
  ADD COLUMN IF NOT EXISTS trigger_reason text;

CREATE UNIQUE INDEX IF NOT EXISTS llm_execution_events_idempotency_key_idx
  ON llm_execution_events (idempotency_key)
  WHERE idempotency_key IS NOT NULL;
