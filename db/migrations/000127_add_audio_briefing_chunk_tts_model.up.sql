ALTER TABLE audio_briefing_script_chunks
  ADD COLUMN IF NOT EXISTS tts_model text NOT NULL DEFAULT '';
