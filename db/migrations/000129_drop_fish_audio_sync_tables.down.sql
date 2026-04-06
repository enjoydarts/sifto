CREATE TABLE IF NOT EXISTS fish_model_sync_runs (
  id BIGSERIAL PRIMARY KEY,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  last_progress_at TIMESTAMPTZ,
  status TEXT NOT NULL CHECK (status IN ('running', 'success', 'failed')),
  trigger_type TEXT NOT NULL CHECK (trigger_type IN ('manual', 'cron')),
  fetched_count INTEGER NOT NULL DEFAULT 0,
  saved_count INTEGER NOT NULL DEFAULT 0,
  error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_fish_model_sync_runs_started_at
  ON fish_model_sync_runs (started_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS fish_model_snapshots (
  id BIGSERIAL PRIMARY KEY,
  sync_run_id BIGINT NOT NULL REFERENCES fish_model_sync_runs(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  cover_image TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL DEFAULT '',
  train_mode TEXT NOT NULL DEFAULT '',
  author_name TEXT NOT NULL DEFAULT '',
  author_avatar TEXT NOT NULL DEFAULT '',
  language_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
  tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  samples_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  like_count INTEGER NOT NULL DEFAULT 0,
  mark_count INTEGER NOT NULL DEFAULT 0,
  shared_count INTEGER NOT NULL DEFAULT 0,
  task_count INTEGER NOT NULL DEFAULT 0,
  sample_count INTEGER NOT NULL DEFAULT 0,
  created_at_remote TIMESTAMPTZ,
  updated_at_remote TIMESTAMPTZ,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fish_model_snapshots_sync_run_id
  ON fish_model_snapshots (sync_run_id);

CREATE INDEX IF NOT EXISTS idx_fish_model_snapshots_model_id
  ON fish_model_snapshots (model_id);

CREATE INDEX IF NOT EXISTS idx_fish_model_snapshots_title
  ON fish_model_snapshots (title);
