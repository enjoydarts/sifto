ALTER TABLE prompt_experiment_arms
  ADD COLUMN IF NOT EXISTS template_id UUID;

UPDATE prompt_experiment_arms AS arms
SET template_id = experiments.template_id
FROM prompt_experiments AS experiments
WHERE experiments.id = arms.experiment_id
  AND arms.template_id IS NULL;

ALTER TABLE prompt_experiment_arms
  ALTER COLUMN template_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'prompt_template_versions_id_template_id_key'
  ) THEN
    ALTER TABLE prompt_template_versions
      ADD CONSTRAINT prompt_template_versions_id_template_id_key UNIQUE (id, template_id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'prompt_experiments_id_template_id_key'
  ) THEN
    ALTER TABLE prompt_experiments
      ADD CONSTRAINT prompt_experiments_id_template_id_key UNIQUE (id, template_id);
  END IF;
END $$;

ALTER TABLE prompt_templates
  DROP CONSTRAINT IF EXISTS prompt_templates_active_version_id_fkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'prompt_templates_active_version_same_template_fkey'
  ) THEN
    ALTER TABLE prompt_templates
      ADD CONSTRAINT prompt_templates_active_version_same_template_fkey
      FOREIGN KEY (active_version_id, id)
      REFERENCES prompt_template_versions(id, template_id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'prompt_experiment_arms_template_id_fkey'
  ) THEN
    ALTER TABLE prompt_experiment_arms
      ADD CONSTRAINT prompt_experiment_arms_template_id_fkey
      FOREIGN KEY (template_id) REFERENCES prompt_templates(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'prompt_experiment_arms_same_experiment_template_fkey'
  ) THEN
    ALTER TABLE prompt_experiment_arms
      ADD CONSTRAINT prompt_experiment_arms_same_experiment_template_fkey
      FOREIGN KEY (experiment_id, template_id)
      REFERENCES prompt_experiments(id, template_id)
      ON DELETE CASCADE;
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'prompt_experiment_arms_same_version_template_fkey'
  ) THEN
    ALTER TABLE prompt_experiment_arms
      ADD CONSTRAINT prompt_experiment_arms_same_version_template_fkey
      FOREIGN KEY (version_id, template_id)
      REFERENCES prompt_template_versions(id, template_id)
      ON DELETE CASCADE;
  END IF;
END $$;
