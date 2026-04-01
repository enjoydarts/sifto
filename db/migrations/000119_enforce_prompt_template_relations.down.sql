ALTER TABLE prompt_experiment_arms
  DROP CONSTRAINT IF EXISTS prompt_experiment_arms_same_version_template_fkey;

ALTER TABLE prompt_experiment_arms
  DROP CONSTRAINT IF EXISTS prompt_experiment_arms_same_experiment_template_fkey;

ALTER TABLE prompt_experiment_arms
  DROP CONSTRAINT IF EXISTS prompt_experiment_arms_template_id_fkey;

ALTER TABLE prompt_templates
  DROP CONSTRAINT IF EXISTS prompt_templates_active_version_same_template_fkey;

ALTER TABLE prompt_templates
  ADD CONSTRAINT prompt_templates_active_version_id_fkey
  FOREIGN KEY (active_version_id) REFERENCES prompt_template_versions(id) ON DELETE SET NULL;

ALTER TABLE prompt_experiments
  DROP CONSTRAINT IF EXISTS prompt_experiments_id_template_id_key;

ALTER TABLE prompt_template_versions
  DROP CONSTRAINT IF EXISTS prompt_template_versions_id_template_id_key;

ALTER TABLE prompt_experiment_arms
  DROP COLUMN IF EXISTS template_id;
