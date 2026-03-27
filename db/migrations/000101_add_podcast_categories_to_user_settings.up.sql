ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS podcast_category TEXT,
  ADD COLUMN IF NOT EXISTS podcast_subcategory TEXT;
