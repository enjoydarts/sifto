ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS chunk_trailing_silence_seconds REAL NOT NULL DEFAULT 1.0;
