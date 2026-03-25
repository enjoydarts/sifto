ALTER TABLE audio_briefing_persona_voices
  ADD COLUMN IF NOT EXISTS emotional_intensity REAL NOT NULL DEFAULT 1.0,
  ADD COLUMN IF NOT EXISTS tempo_dynamics REAL NOT NULL DEFAULT 1.0,
  ADD COLUMN IF NOT EXISTS line_break_silence_seconds REAL NOT NULL DEFAULT 0.4;
