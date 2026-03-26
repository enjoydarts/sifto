ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS archive_status TEXT NOT NULL DEFAULT 'active';

UPDATE audio_briefing_jobs
SET archive_status = 'active'
WHERE archive_status IS NULL OR archive_status = '';

ALTER TABLE audio_briefing_jobs
  DROP CONSTRAINT IF EXISTS audio_briefing_jobs_archive_status_check;

ALTER TABLE audio_briefing_jobs
  ADD CONSTRAINT audio_briefing_jobs_archive_status_check
  CHECK (archive_status IN ('active', 'archived'));

CREATE INDEX IF NOT EXISTS idx_audio_briefing_jobs_archive_status
  ON audio_briefing_jobs (user_id, archive_status, slot_started_at_jst DESC, created_at DESC);
