CREATE INDEX IF NOT EXISTS idx_sources_user_id
  ON sources (user_id);

CREATE INDEX IF NOT EXISTS idx_items_source_status_effective_published_at
  ON items (source_id, status, (COALESCE(published_at, created_at)) DESC);

CREATE INDEX IF NOT EXISTS idx_items_source_effective_published_at_summarized
  ON items (source_id, (COALESCE(published_at, created_at)) DESC)
  WHERE status = 'summarized';
