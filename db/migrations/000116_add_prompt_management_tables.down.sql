ALTER TABLE llm_execution_events
  DROP COLUMN IF EXISTS prompt_experiment_arm_id,
  DROP COLUMN IF EXISTS prompt_experiment_id,
  DROP COLUMN IF EXISTS prompt_version_number,
  DROP COLUMN IF EXISTS prompt_version_id,
  DROP COLUMN IF EXISTS prompt_source,
  DROP COLUMN IF EXISTS prompt_key;

ALTER TABLE llm_usage_logs
  DROP COLUMN IF EXISTS prompt_experiment_arm_id,
  DROP COLUMN IF EXISTS prompt_experiment_id,
  DROP COLUMN IF EXISTS prompt_version_number,
  DROP COLUMN IF EXISTS prompt_version_id,
  DROP COLUMN IF EXISTS prompt_source,
  DROP COLUMN IF EXISTS prompt_key;

DROP TABLE IF EXISTS prompt_admin_audit_logs;
DROP TABLE IF EXISTS prompt_experiment_arms;
DROP TABLE IF EXISTS prompt_experiments;

ALTER TABLE prompt_templates
  DROP CONSTRAINT IF EXISTS prompt_templates_active_version_id_fkey;

DROP TABLE IF EXISTS prompt_template_versions;
DROP TABLE IF EXISTS prompt_templates;
