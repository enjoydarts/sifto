ALTER TABLE audio_briefing_script_chunks
  ADD COLUMN IF NOT EXISTS speaker TEXT;

ALTER TABLE audio_briefing_script_chunks
  DROP CONSTRAINT IF EXISTS audio_briefing_script_chunks_speaker_check;

ALTER TABLE audio_briefing_script_chunks
  ADD CONSTRAINT audio_briefing_script_chunks_speaker_check
  CHECK (speaker IS NULL OR speaker IN ('host', 'partner'));
