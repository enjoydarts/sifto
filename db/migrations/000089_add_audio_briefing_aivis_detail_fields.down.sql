ALTER TABLE audio_briefing_persona_voices
  DROP COLUMN IF EXISTS line_break_silence_seconds,
  DROP COLUMN IF EXISTS tempo_dynamics,
  DROP COLUMN IF EXISTS emotional_intensity;
