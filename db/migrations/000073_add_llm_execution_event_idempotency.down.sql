DROP INDEX IF EXISTS llm_execution_events_idempotency_key_idx;

ALTER TABLE llm_execution_events
  DROP COLUMN IF EXISTS trigger_reason,
  DROP COLUMN IF EXISTS trigger_id,
  DROP COLUMN IF EXISTS idempotency_key;
