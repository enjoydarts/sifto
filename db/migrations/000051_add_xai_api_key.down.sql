ALTER TABLE user_settings
  DROP COLUMN IF EXISTS xai_api_key_last4,
  DROP COLUMN IF EXISTS xai_api_key_enc;
