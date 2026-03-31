ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS conversation_mode TEXT NOT NULL DEFAULT 'single';

ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_conversation_mode_check;

ALTER TABLE audio_briefing_settings
  ADD CONSTRAINT audio_briefing_settings_conversation_mode_check
  CHECK (conversation_mode IN ('single', 'duo'));

ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS conversation_mode TEXT NOT NULL DEFAULT 'single',
  ADD COLUMN IF NOT EXISTS partner_persona TEXT,
  ADD COLUMN IF NOT EXISTS pipeline_stage TEXT;

ALTER TABLE audio_briefing_jobs
  DROP CONSTRAINT IF EXISTS audio_briefing_jobs_conversation_mode_check;

ALTER TABLE audio_briefing_jobs
  ADD CONSTRAINT audio_briefing_jobs_conversation_mode_check
  CHECK (conversation_mode IN ('single', 'duo'));
