ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS prompt_key TEXT NULL,
  ADD COLUMN IF NOT EXISTS prompt_source TEXT NULL,
  ADD COLUMN IF NOT EXISTS prompt_version_id UUID NULL REFERENCES prompt_template_versions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS prompt_version_number INTEGER NULL,
  ADD COLUMN IF NOT EXISTS prompt_experiment_id UUID NULL REFERENCES prompt_experiments(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS prompt_experiment_arm_id UUID NULL REFERENCES prompt_experiment_arms(id) ON DELETE SET NULL;
