CREATE TABLE IF NOT EXISTS playback_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  subtitle TEXT NOT NULL DEFAULT '',
  current_position_sec INTEGER NOT NULL DEFAULT 0,
  duration_sec INTEGER NOT NULL DEFAULT 0,
  progress_ratio DOUBLE PRECISION,
  resume_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_playback_sessions_user_mode_updated_at
  ON playback_sessions (user_id, mode, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_playback_sessions_user_updated_at
  ON playback_sessions (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS playback_events (
  id UUID PRIMARY KEY,
  session_id UUID NOT NULL REFERENCES playback_sessions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mode TEXT NOT NULL,
  event_type TEXT NOT NULL,
  position_sec INTEGER NOT NULL DEFAULT 0,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_playback_events_session_created_at
  ON playback_events (session_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_playback_events_user_created_at
  ON playback_events (user_id, created_at DESC);
