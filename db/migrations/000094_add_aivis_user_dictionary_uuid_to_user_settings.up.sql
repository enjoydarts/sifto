ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS aivis_user_dictionary_uuid TEXT;
