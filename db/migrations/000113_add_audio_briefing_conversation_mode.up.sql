ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS conversation_mode TEXT NOT NULL DEFAULT 'single';

UPDATE audio_briefing_settings
SET conversation_mode = 'single'
WHERE conversation_mode IS NULL;

ALTER TABLE audio_briefing_settings
  ALTER COLUMN conversation_mode SET DEFAULT 'single',
  ALTER COLUMN conversation_mode SET NOT NULL;

ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_conversation_mode_check;

ALTER TABLE audio_briefing_settings
  ADD CONSTRAINT audio_briefing_settings_conversation_mode_check
  CHECK (conversation_mode IN ('single', 'duo'));

ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS conversation_mode TEXT NOT NULL DEFAULT 'single',
  ADD COLUMN IF NOT EXISTS partner_persona TEXT,
  ADD COLUMN IF NOT EXISTS pipeline_stage TEXT;

UPDATE audio_briefing_jobs
SET conversation_mode = 'single'
WHERE conversation_mode IS NULL;

ALTER TABLE audio_briefing_jobs
  ALTER COLUMN conversation_mode SET DEFAULT 'single',
  ALTER COLUMN conversation_mode SET NOT NULL;

ALTER TABLE audio_briefing_jobs
  DROP CONSTRAINT IF EXISTS audio_briefing_jobs_conversation_mode_check;

ALTER TABLE audio_briefing_jobs
  ADD CONSTRAINT audio_briefing_jobs_conversation_mode_check
  CHECK (conversation_mode IN ('single', 'duo'));
