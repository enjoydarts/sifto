ALTER TABLE audio_briefing_settings
  ADD COLUMN bgm_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN bgm_r2_prefix TEXT;

ALTER TABLE audio_briefing_jobs
  ADD COLUMN bgm_object_key TEXT;
