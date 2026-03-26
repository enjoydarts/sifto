DROP INDEX IF EXISTS idx_audio_briefing_jobs_archive_status;

ALTER TABLE audio_briefing_jobs
  DROP CONSTRAINT IF EXISTS audio_briefing_jobs_archive_status_check;

ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS archive_status;
