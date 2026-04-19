ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS featherless_api_key_enc TEXT,
  ADD COLUMN IF NOT EXISTS featherless_api_key_last4 TEXT;

ALTER TABLE provider_model_change_events
  DROP CONSTRAINT IF EXISTS provider_model_change_events_change_type_check;

ALTER TABLE provider_model_change_events
  ADD CONSTRAINT provider_model_change_events_change_type_check
  CHECK (change_type IN ('added', 'constrained', 'availability_changed', 'gated_changed', 'removed'));

CREATE TABLE IF NOT EXISTS featherless_model_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  last_progress_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'running',
  trigger_type TEXT NOT NULL DEFAULT 'manual',
  fetched_count INTEGER NOT NULL DEFAULT 0,
  accepted_count INTEGER NOT NULL DEFAULT 0,
  error_message TEXT
);

CREATE TABLE IF NOT EXISTS featherless_model_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sync_run_id UUID NOT NULL REFERENCES featherless_model_sync_runs(id) ON DELETE CASCADE,
  fetched_at TIMESTAMPTZ NOT NULL,
  model_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  model_class TEXT NOT NULL DEFAULT '',
  context_length INTEGER,
  max_completion_tokens INTEGER,
  is_gated BOOLEAN NOT NULL DEFAULT FALSE,
  available_on_current_plan BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS featherless_model_snapshots_sync_run_idx
  ON featherless_model_snapshots(sync_run_id);

CREATE INDEX IF NOT EXISTS featherless_model_snapshots_fetched_at_idx
  ON featherless_model_snapshots(fetched_at DESC);

CREATE INDEX IF NOT EXISTS featherless_model_snapshots_model_id_idx
  ON featherless_model_snapshots(model_id);
