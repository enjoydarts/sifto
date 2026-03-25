DROP INDEX IF EXISTS idx_user_settings_podcast_feed_slug;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS podcast_artwork_url,
  DROP COLUMN IF EXISTS podcast_explicit,
  DROP COLUMN IF EXISTS podcast_language,
  DROP COLUMN IF EXISTS podcast_author,
  DROP COLUMN IF EXISTS podcast_description,
  DROP COLUMN IF EXISTS podcast_title,
  DROP COLUMN IF EXISTS podcast_feed_slug,
  DROP COLUMN IF EXISTS podcast_enabled;
