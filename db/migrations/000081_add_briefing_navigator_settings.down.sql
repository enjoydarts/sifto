ALTER TABLE user_settings
  DROP COLUMN IF EXISTS navigator_fallback_model,
  DROP COLUMN IF EXISTS navigator_model,
  DROP COLUMN IF EXISTS navigator_persona,
  DROP COLUMN IF EXISTS navigator_enabled;
