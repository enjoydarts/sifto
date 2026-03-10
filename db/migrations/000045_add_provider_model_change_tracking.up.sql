CREATE TABLE IF NOT EXISTS provider_model_snapshots (
  provider TEXT PRIMARY KEY,
  models JSONB NOT NULL DEFAULT '[]'::jsonb,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  status TEXT NOT NULL DEFAULT 'ok',
  error TEXT
);

CREATE TABLE IF NOT EXISTS provider_model_change_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider TEXT NOT NULL,
  change_type TEXT NOT NULL CHECK (change_type IN ('added', 'removed')),
  model_id TEXT NOT NULL,
  detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_provider_model_change_events_detected_at
  ON provider_model_change_events (detected_at DESC);

CREATE INDEX IF NOT EXISTS idx_provider_model_change_events_provider_detected_at
  ON provider_model_change_events (provider, detected_at DESC);
