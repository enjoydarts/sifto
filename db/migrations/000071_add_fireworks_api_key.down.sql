ALTER TABLE user_settings
  DROP COLUMN IF EXISTS fireworks_api_key_last4,
  DROP COLUMN IF EXISTS fireworks_api_key_enc;
