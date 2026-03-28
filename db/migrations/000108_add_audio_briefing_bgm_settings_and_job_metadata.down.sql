ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS bgm_object_key;

ALTER TABLE audio_briefing_settings
  DROP COLUMN IF EXISTS bgm_r2_prefix,
  DROP COLUMN IF EXISTS bgm_enabled;
