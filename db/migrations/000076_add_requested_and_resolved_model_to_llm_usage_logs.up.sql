ALTER TABLE llm_usage_logs
    ADD COLUMN requested_model TEXT,
    ADD COLUMN resolved_model TEXT;

UPDATE llm_usage_logs
SET requested_model = model
WHERE requested_model IS NULL;
