DROP INDEX IF EXISTS idx_item_summaries_personal_score_item_id;

ALTER TABLE item_summaries
  DROP COLUMN IF EXISTS personal_score_reason,
  DROP COLUMN IF EXISTS personal_score;
