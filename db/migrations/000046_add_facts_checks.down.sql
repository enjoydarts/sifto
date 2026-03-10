ALTER TABLE llm_usage_logs
  DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
  ADD CONSTRAINT llm_usage_logs_purpose_check
  CHECK (purpose IN ('facts', 'summary', 'digest', 'embedding', 'source_suggestion', 'digest_cluster_draft', 'ask', 'faithfulness_check'));

DROP INDEX IF EXISTS idx_item_facts_checks_item_id;

DROP TABLE IF EXISTS item_facts_checks;

ALTER TABLE user_settings
DROP COLUMN IF EXISTS facts_check_model;
