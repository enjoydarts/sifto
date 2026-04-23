DROP INDEX IF EXISTS deepinfra_model_snapshots_provider_slug_idx;

ALTER TABLE deepinfra_model_snapshots
  DROP COLUMN IF EXISTS tags_json,
  DROP COLUMN IF EXISTS cache_read_per_mtok_usd,
  DROP COLUMN IF EXISTS max_tokens,
  DROP COLUMN IF EXISTS description_ja,
  DROP COLUMN IF EXISTS description_en,
  DROP COLUMN IF EXISTS provider_slug;

ALTER TABLE deepinfra_model_sync_runs
  DROP COLUMN IF EXISTS last_error_message,
  DROP COLUMN IF EXISTS translation_failed_count,
  DROP COLUMN IF EXISTS translation_completed_count,
  DROP COLUMN IF EXISTS translation_target_count;
