ALTER TABLE audio_briefing_script_chunks
  DROP CONSTRAINT IF EXISTS audio_briefing_script_chunks_speaker_check;

ALTER TABLE audio_briefing_script_chunks
  DROP COLUMN IF EXISTS speaker;
