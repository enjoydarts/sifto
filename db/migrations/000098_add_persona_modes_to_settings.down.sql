ALTER TABLE audio_briefing_settings
  DROP COLUMN IF EXISTS default_persona_mode;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS navigator_persona_mode;
