DROP INDEX IF EXISTS idx_summary_faithfulness_checks_item_id;

DROP TABLE IF EXISTS summary_faithfulness_checks;

ALTER TABLE user_settings
DROP COLUMN IF EXISTS faithfulness_check_model;
