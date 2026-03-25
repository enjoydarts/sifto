ALTER TABLE user_settings
  DROP COLUMN IF EXISTS audio_briefing_script_fallback_model,
  DROP COLUMN IF EXISTS audio_briefing_script_model;
