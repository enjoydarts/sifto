ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS podcast_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS podcast_feed_slug TEXT,
  ADD COLUMN IF NOT EXISTS podcast_title TEXT,
  ADD COLUMN IF NOT EXISTS podcast_description TEXT,
  ADD COLUMN IF NOT EXISTS podcast_author TEXT,
  ADD COLUMN IF NOT EXISTS podcast_language TEXT NOT NULL DEFAULT 'ja',
  ADD COLUMN IF NOT EXISTS podcast_explicit BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS podcast_artwork_url TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_settings_podcast_feed_slug
  ON user_settings (podcast_feed_slug)
  WHERE podcast_feed_slug IS NOT NULL AND podcast_feed_slug <> '';
