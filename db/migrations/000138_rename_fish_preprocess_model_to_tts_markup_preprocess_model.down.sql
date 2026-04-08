DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'user_settings'
      AND column_name = 'tts_markup_preprocess_model'
  ) AND NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'user_settings'
      AND column_name = 'fish_preprocess_model'
  ) THEN
    ALTER TABLE user_settings
      RENAME COLUMN tts_markup_preprocess_model TO fish_preprocess_model;
  END IF;
END $$;
