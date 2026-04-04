ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS schedule_mode TEXT;

UPDATE audio_briefing_settings
SET schedule_mode = 'interval'
WHERE schedule_mode IS NULL;

ALTER TABLE audio_briefing_settings
  ALTER COLUMN schedule_mode SET DEFAULT 'interval';

ALTER TABLE audio_briefing_settings
  ALTER COLUMN schedule_mode SET NOT NULL;

ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_schedule_mode_check;

ALTER TABLE audio_briefing_settings
  ADD CONSTRAINT audio_briefing_settings_schedule_mode_check
  CHECK (schedule_mode IN ('interval', 'fixed_slots_3x'));
