-- Improve item list/topic filter and score ordering access paths.
CREATE INDEX IF NOT EXISTS idx_item_summaries_topics_gin
  ON item_summaries USING GIN (topics);

CREATE INDEX IF NOT EXISTS idx_item_summaries_score_item
  ON item_summaries (score DESC, item_id);
