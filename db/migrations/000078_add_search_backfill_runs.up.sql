CREATE TABLE IF NOT EXISTS search_backfill_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  requested_offset INTEGER NOT NULL DEFAULT 0,
  batch_size INTEGER NOT NULL DEFAULT 100,
  all_items BOOLEAN NOT NULL DEFAULT FALSE,
  total_items INTEGER NOT NULL DEFAULT 0,
  queued_batches INTEGER NOT NULL DEFAULT 0,
  completed_batches INTEGER NOT NULL DEFAULT 0,
  failed_batches INTEGER NOT NULL DEFAULT 0,
  processed_items INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'queued',
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_search_backfill_runs_created_at
  ON search_backfill_runs (created_at DESC);
