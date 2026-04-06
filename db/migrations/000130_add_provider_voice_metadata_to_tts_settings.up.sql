ALTER TABLE audio_briefing_persona_voices
  ADD COLUMN provider_voice_label TEXT,
  ADD COLUMN provider_voice_description TEXT;

ALTER TABLE summary_audio_voice_settings
  ADD COLUMN provider_voice_label TEXT,
  ADD COLUMN provider_voice_description TEXT;
