-- Items / reading-plan joins and existence checks
CREATE INDEX IF NOT EXISTS idx_items_source_created_at
  ON items (source_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_items_source_status_created_at
  ON items (source_id, status, created_at DESC);

-- Fast exists/joins by (user_id, item_id)
CREATE INDEX IF NOT EXISTS idx_item_reads_user_item
  ON item_reads (user_id, item_id);

CREATE INDEX IF NOT EXISTS idx_item_feedbacks_user_item
  ON item_feedbacks (user_id, item_id);

CREATE INDEX IF NOT EXISTS idx_item_feedbacks_user_favorite_item
  ON item_feedbacks (user_id, is_favorite, item_id);

-- Dashboard latest digests
CREATE INDEX IF NOT EXISTS idx_digests_user_digest_date
  ON digests (user_id, digest_date DESC);

-- LLM usage summaries with purpose grouping/filtering
CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_user_purpose_created_at
  ON llm_usage_logs (user_id, purpose, created_at DESC);
