ALTER TABLE openrouter_model_sync_runs
  ADD COLUMN IF NOT EXISTS last_progress_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS translation_failed_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error_message TEXT;
