ALTER TABLE audio_briefing_settings
  ADD COLUMN IF NOT EXISTS chunk_trailing_silence_seconds REAL NOT NULL DEFAULT 1.0;

UPDATE audio_briefing_settings
SET chunk_trailing_silence_seconds = 1.0
WHERE chunk_trailing_silence_seconds IS NULL;

ALTER TABLE audio_briefing_settings
  ALTER COLUMN chunk_trailing_silence_seconds SET DEFAULT 1.0,
  ALTER COLUMN chunk_trailing_silence_seconds SET NOT NULL;

ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_chunk_trailing_silence_seconds_check;

ALTER TABLE audio_briefing_settings
  ADD CONSTRAINT audio_briefing_settings_chunk_trailing_silence_seconds_check
  CHECK (chunk_trailing_silence_seconds >= 0 AND chunk_trailing_silence_seconds <= 5);
