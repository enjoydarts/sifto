ALTER TABLE summary_audio_voice_settings
  DROP COLUMN IF EXISTS provider_voice_description,
  DROP COLUMN IF EXISTS provider_voice_label;

ALTER TABLE audio_briefing_persona_voices
  DROP COLUMN IF EXISTS provider_voice_description,
  DROP COLUMN IF EXISTS provider_voice_label;
