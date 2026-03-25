ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS podcast_public_object_key TEXT,
  ADD COLUMN IF NOT EXISTS podcast_public_bucket TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS podcast_public_deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_audio_briefing_jobs_podcast_public_object_key
  ON audio_briefing_jobs (podcast_public_object_key)
  WHERE podcast_public_object_key IS NOT NULL AND podcast_public_object_key <> '';
