ALTER TABLE openrouter_model_sync_runs
  ADD COLUMN IF NOT EXISTS trigger_type TEXT NOT NULL DEFAULT 'cron',
  ADD COLUMN IF NOT EXISTS translation_target_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS translation_completed_count INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS openrouter_model_sync_runs_manual_running_idx
  ON openrouter_model_sync_runs(trigger_type, status, started_at DESC);
