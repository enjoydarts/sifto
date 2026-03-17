ALTER TABLE openrouter_model_sync_runs
    DROP COLUMN IF EXISTS last_error_message,
    DROP COLUMN IF EXISTS translation_failed_count,
    DROP COLUMN IF EXISTS last_progress_at;
