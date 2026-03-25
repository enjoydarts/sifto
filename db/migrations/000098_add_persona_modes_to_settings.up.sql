ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS navigator_persona_mode TEXT NOT NULL DEFAULT 'fixed';

ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS default_persona_mode TEXT NOT NULL DEFAULT 'fixed';
