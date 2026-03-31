ALTER TABLE audio_briefing_jobs
  DROP CONSTRAINT IF EXISTS audio_briefing_jobs_conversation_mode_check;

ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS pipeline_stage,
  DROP COLUMN IF EXISTS partner_persona,
  DROP COLUMN IF EXISTS conversation_mode;

ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_conversation_mode_check;

ALTER TABLE audio_briefing_settings
  DROP COLUMN IF EXISTS conversation_mode;
