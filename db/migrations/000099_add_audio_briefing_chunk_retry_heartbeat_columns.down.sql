DROP INDEX IF EXISTS idx_audio_briefing_script_chunks_last_heartbeat_at;

ALTER TABLE audio_briefing_script_chunks
  DROP CONSTRAINT IF EXISTS audio_briefing_script_chunks_tts_status_check;

ALTER TABLE audio_briefing_script_chunks
  DROP COLUMN IF EXISTS completed_at,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS last_heartbeat_at,
  DROP COLUMN IF EXISTS heartbeat_token,
  DROP COLUMN IF EXISTS last_error_code,
  DROP COLUMN IF EXISTS attempt_count;

ALTER TABLE audio_briefing_script_chunks
  ADD CONSTRAINT audio_briefing_script_chunks_tts_status_check
  CHECK (tts_status IN ('pending', 'generating', 'generated', 'failed'));
