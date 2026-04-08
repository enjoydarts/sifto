ALTER TABLE item_summaries
  ADD COLUMN personal_score DOUBLE PRECISION,
  ADD COLUMN personal_score_reason TEXT;

CREATE INDEX idx_item_summaries_personal_score_item_id
  ON item_summaries (personal_score DESC NULLS LAST, item_id);
