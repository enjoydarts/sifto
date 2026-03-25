ALTER TABLE audio_briefing_script_chunks
  DROP COLUMN IF EXISTS r2_storage_bucket;

ALTER TABLE audio_briefing_jobs
  DROP COLUMN IF EXISTS r2_storage_bucket;
