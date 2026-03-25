ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS audio_briefing_script_model TEXT,
  ADD COLUMN IF NOT EXISTS audio_briefing_script_fallback_model TEXT;
