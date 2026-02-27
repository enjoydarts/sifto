CREATE TABLE IF NOT EXISTS source_health_snapshots (
  source_id UUID PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
  total_items INT NOT NULL DEFAULT 0,
  failed_items INT NOT NULL DEFAULT 0,
  summarized_items INT NOT NULL DEFAULT 0,
  failure_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
  last_item_at TIMESTAMPTZ,
  last_fetched_at TIMESTAMPTZ,
  status TEXT NOT NULL,
  reason TEXT,
  checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_source_health_snapshots_status_checked_at
  ON source_health_snapshots (status, checked_at DESC);
