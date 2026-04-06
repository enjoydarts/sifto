ALTER TABLE audio_briefing_script_chunks
  ADD COLUMN IF NOT EXISTS preprocessed_text TEXT;
