CREATE TABLE IF NOT EXISTS poe_usage_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'running',
  sync_source TEXT NOT NULL DEFAULT 'manual',
  fetched_count INTEGER NOT NULL DEFAULT 0,
  inserted_count INTEGER NOT NULL DEFAULT 0,
  updated_count INTEGER NOT NULL DEFAULT 0,
  latest_entry_at TIMESTAMPTZ,
  oldest_entry_at TIMESTAMPTZ,
  error_message TEXT
);

CREATE INDEX IF NOT EXISTS poe_usage_sync_runs_user_started_idx
  ON poe_usage_sync_runs(user_id, started_at DESC);

CREATE TABLE IF NOT EXISTS poe_usage_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  sync_run_id UUID REFERENCES poe_usage_sync_runs(id) ON DELETE SET NULL,
  query_id TEXT NOT NULL,
  bot_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  cost_usd NUMERIC(12, 6) NOT NULL DEFAULT 0,
  raw_cost_usd TEXT NOT NULL DEFAULT '',
  cost_points INTEGER NOT NULL DEFAULT 0,
  cost_breakdown_in_points JSONB NOT NULL DEFAULT '{}'::jsonb,
  usage_type TEXT NOT NULL DEFAULT '',
  chat_name TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS poe_usage_entries_user_query_id_idx
  ON poe_usage_entries(user_id, query_id);

CREATE INDEX IF NOT EXISTS poe_usage_entries_user_created_idx
  ON poe_usage_entries(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS poe_usage_entries_user_bot_created_idx
  ON poe_usage_entries(user_id, bot_name, created_at DESC);
