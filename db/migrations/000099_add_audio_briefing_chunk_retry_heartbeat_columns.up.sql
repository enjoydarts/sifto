ALTER TABLE audio_briefing_script_chunks
  DROP CONSTRAINT IF EXISTS audio_briefing_script_chunks_tts_status_check;

ALTER TABLE audio_briefing_script_chunks
  ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error_code TEXT,
  ADD COLUMN IF NOT EXISTS heartbeat_token TEXT,
  ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

ALTER TABLE audio_briefing_script_chunks
  ADD CONSTRAINT audio_briefing_script_chunks_tts_status_check
  CHECK (tts_status IN ('pending', 'generating', 'retry_wait', 'generated', 'failed', 'exhausted'));

CREATE INDEX IF NOT EXISTS idx_audio_briefing_script_chunks_last_heartbeat_at
  ON audio_briefing_script_chunks (last_heartbeat_at DESC)
  WHERE tts_status = 'generating';
