DROP INDEX IF EXISTS openrouter_model_sync_runs_manual_running_idx;

ALTER TABLE openrouter_model_sync_runs
  DROP COLUMN IF EXISTS translation_completed_count,
  DROP COLUMN IF EXISTS translation_target_count,
  DROP COLUMN IF EXISTS trigger_type;
