ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS poe_api_key_enc TEXT,
  ADD COLUMN IF NOT EXISTS poe_api_key_last4 TEXT;

CREATE TABLE IF NOT EXISTS poe_model_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  last_progress_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'running',
  trigger_type TEXT NOT NULL DEFAULT 'manual',
  fetched_count INTEGER NOT NULL DEFAULT 0,
  accepted_count INTEGER NOT NULL DEFAULT 0,
  translation_target_count INTEGER NOT NULL DEFAULT 0,
  translation_completed_count INTEGER NOT NULL DEFAULT 0,
  translation_failed_count INTEGER NOT NULL DEFAULT 0,
  last_error_message TEXT,
  error_message TEXT
);

CREATE TABLE IF NOT EXISTS poe_model_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sync_run_id UUID NOT NULL REFERENCES poe_model_sync_runs(id) ON DELETE CASCADE,
  fetched_at TIMESTAMPTZ NOT NULL,
  model_id TEXT NOT NULL,
  canonical_slug TEXT,
  display_name TEXT NOT NULL,
  owned_by TEXT NOT NULL DEFAULT '',
  description_en TEXT,
  description_ja TEXT,
  context_length INTEGER,
  pricing_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  architecture_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  modality_flags_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  transport_supports_openai_compat BOOLEAN NOT NULL DEFAULT TRUE,
  transport_supports_anthropic_compat BOOLEAN NOT NULL DEFAULT FALSE,
  preferred_transport TEXT NOT NULL DEFAULT 'openai'
);

CREATE INDEX IF NOT EXISTS poe_model_snapshots_sync_run_idx
  ON poe_model_snapshots(sync_run_id);

CREATE INDEX IF NOT EXISTS poe_model_snapshots_fetched_at_idx
  ON poe_model_snapshots(fetched_at DESC);

CREATE INDEX IF NOT EXISTS poe_model_snapshots_model_id_idx
  ON poe_model_snapshots(model_id);
