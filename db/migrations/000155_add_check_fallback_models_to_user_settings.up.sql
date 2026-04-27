ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS facts_check_fallback_model text,
  ADD COLUMN IF NOT EXISTS faithfulness_check_fallback_model text;
