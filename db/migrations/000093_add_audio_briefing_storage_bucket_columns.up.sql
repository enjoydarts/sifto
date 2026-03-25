ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS r2_storage_bucket TEXT NOT NULL DEFAULT '';

ALTER TABLE audio_briefing_script_chunks
  ADD COLUMN IF NOT EXISTS r2_storage_bucket TEXT NOT NULL DEFAULT '';
