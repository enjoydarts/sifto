ALTER TABLE llm_usage_logs
    DROP COLUMN IF EXISTS resolved_model,
    DROP COLUMN IF EXISTS requested_model;
