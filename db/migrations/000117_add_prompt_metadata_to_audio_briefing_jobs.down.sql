ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS prompt_experiment_arm_id,
  DROP COLUMN IF EXISTS prompt_experiment_id,
  DROP COLUMN IF EXISTS prompt_version_number,
  DROP COLUMN IF EXISTS prompt_version_id,
  DROP COLUMN IF EXISTS prompt_source,
  DROP COLUMN IF EXISTS prompt_key;
