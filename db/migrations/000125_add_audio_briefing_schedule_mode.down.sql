ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_schedule_mode_check;

ALTER TABLE audio_briefing_settings
  DROP COLUMN IF EXISTS schedule_mode;
