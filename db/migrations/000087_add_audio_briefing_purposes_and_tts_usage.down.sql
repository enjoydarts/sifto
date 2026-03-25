DROP INDEX IF EXISTS idx_tts_usage_logs_job_id;
DROP INDEX IF EXISTS idx_tts_usage_logs_user_created_at;
DROP TABLE IF EXISTS tts_usage_logs;

ALTER TABLE llm_usage_logs
  DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
  ADD CONSTRAINT llm_usage_logs_purpose_check
  CHECK (purpose IN (
    'facts',
    'facts_localization',
    'facts_check',
    'summary',
    'digest',
    'embedding',
    'source_suggestion',
    'digest_cluster_draft',
    'ask',
    'faithfulness_check',
    'briefing_navigator',
    'item_navigator',
    'source_navigator',
    'ask_navigator'
  ));
