ALTER TABLE user_settings
  DROP COLUMN IF EXISTS podcast_subcategory,
  DROP COLUMN IF EXISTS podcast_category;
