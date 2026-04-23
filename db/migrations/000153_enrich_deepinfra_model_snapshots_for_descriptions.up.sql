ALTER TABLE deepinfra_model_sync_runs
  ADD COLUMN IF NOT EXISTS translation_target_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS translation_completed_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS translation_failed_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error_message TEXT;

ALTER TABLE deepinfra_model_snapshots
  ADD COLUMN IF NOT EXISTS provider_slug TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS description_en TEXT,
  ADD COLUMN IF NOT EXISTS description_ja TEXT,
  ADD COLUMN IF NOT EXISTS max_tokens INTEGER,
  ADD COLUMN IF NOT EXISTS cache_read_per_mtok_usd DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS tags_json JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS deepinfra_model_snapshots_provider_slug_idx
  ON deepinfra_model_snapshots(provider_slug);
