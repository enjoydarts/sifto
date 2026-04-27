ALTER TABLE user_settings
  DROP COLUMN IF EXISTS faithfulness_check_fallback_model,
  DROP COLUMN IF EXISTS facts_check_fallback_model;
