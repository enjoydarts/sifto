ALTER TABLE llm_usage_logs
DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
ADD CONSTRAINT llm_usage_logs_purpose_check
CHECK (purpose IN ('facts', 'summary', 'digest', 'embedding', 'source_suggestion'));
