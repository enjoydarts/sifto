CREATE TABLE IF NOT EXISTS briefing_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  briefing_date DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'ready', 'stale')),
  payload_json JSONB,
  generated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, briefing_date)
);

CREATE INDEX IF NOT EXISTS idx_briefing_snapshots_user_date
  ON briefing_snapshots(user_id, briefing_date DESC);

