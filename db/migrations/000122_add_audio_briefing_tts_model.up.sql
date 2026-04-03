ALTER TABLE audio_briefing_persona_voices
  ADD COLUMN IF NOT EXISTS tts_model text NOT NULL DEFAULT '';
