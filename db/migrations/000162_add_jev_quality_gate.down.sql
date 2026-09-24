DROP TABLE IF EXISTS item_quality_evaluations;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS jev_api_key_last4,
  DROP COLUMN IF EXISTS jev_api_key_enc;
