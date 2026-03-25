CREATE TABLE IF NOT EXISTS aivis_model_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  last_progress_at TIMESTAMPTZ,
  status TEXT NOT NULL CHECK (status IN ('running', 'success', 'failed')),
  trigger_type TEXT NOT NULL CHECK (trigger_type IN ('manual', 'cron')),
  fetched_count INTEGER NOT NULL DEFAULT 0,
  accepted_count INTEGER NOT NULL DEFAULT 0,
  error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_aivis_model_sync_runs_started_at
  ON aivis_model_sync_runs (started_at DESC);

CREATE TABLE IF NOT EXISTS aivis_model_snapshots (
  sync_run_id UUID NOT NULL REFERENCES aivis_model_sync_runs(id) ON DELETE CASCADE,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  aivm_model_uuid TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  detailed_description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT '',
  voice_timbre TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL DEFAULT '',
  is_tag_locked BOOLEAN NOT NULL DEFAULT FALSE,
  total_download_count INTEGER NOT NULL DEFAULT 0,
  like_count INTEGER NOT NULL DEFAULT 0,
  is_liked BOOLEAN NOT NULL DEFAULT FALSE,
  user_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  model_files_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  speakers_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  model_file_count INTEGER NOT NULL DEFAULT 0,
  speaker_count INTEGER NOT NULL DEFAULT 0,
  style_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (sync_run_id, aivm_model_uuid)
);

CREATE INDEX IF NOT EXISTS idx_aivis_model_snapshots_uuid
  ON aivis_model_snapshots (aivm_model_uuid);

CREATE INDEX IF NOT EXISTS idx_aivis_model_snapshots_name
  ON aivis_model_snapshots (name);
