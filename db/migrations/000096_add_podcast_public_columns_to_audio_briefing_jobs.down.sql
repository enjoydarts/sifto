DROP INDEX IF EXISTS idx_audio_briefing_jobs_podcast_public_object_key;

ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS podcast_public_deleted_at,
  DROP COLUMN IF EXISTS podcast_public_bucket,
  DROP COLUMN IF EXISTS podcast_public_object_key;
