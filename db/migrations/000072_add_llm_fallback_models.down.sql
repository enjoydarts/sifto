ALTER TABLE user_settings
  DROP COLUMN IF EXISTS summary_fallback_model,
  DROP COLUMN IF EXISTS facts_fallback_model;
