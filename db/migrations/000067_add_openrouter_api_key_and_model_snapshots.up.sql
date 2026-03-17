ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS openrouter_api_key_enc TEXT,
  ADD COLUMN IF NOT EXISTS openrouter_api_key_last4 TEXT;

CREATE TABLE IF NOT EXISTS openrouter_model_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'running',
  fetched_count INT NOT NULL DEFAULT 0,
  accepted_count INT NOT NULL DEFAULT 0,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS openrouter_model_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sync_run_id UUID NOT NULL REFERENCES openrouter_model_sync_runs(id) ON DELETE CASCADE,
  fetched_at TIMESTAMPTZ NOT NULL,
  model_id TEXT NOT NULL,
  canonical_slug TEXT,
  provider_slug TEXT NOT NULL,
  display_name TEXT NOT NULL,
  description_en TEXT,
  description_ja TEXT,
  context_length INT,
  pricing_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  supported_parameters_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  architecture_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  top_provider_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  modality_flags_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_text_generation BOOLEAN NOT NULL DEFAULT TRUE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS openrouter_model_snapshots_sync_run_idx
  ON openrouter_model_snapshots(sync_run_id);

CREATE INDEX IF NOT EXISTS openrouter_model_snapshots_fetched_at_idx
  ON openrouter_model_snapshots(fetched_at DESC);

CREATE INDEX IF NOT EXISTS openrouter_model_snapshots_model_id_idx
  ON openrouter_model_snapshots(model_id);

CREATE TABLE IF NOT EXISTS openrouter_model_notification_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sync_run_id UUID NOT NULL REFERENCES openrouter_model_sync_runs(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  day_jst DATE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (model_id, day_jst)
);
