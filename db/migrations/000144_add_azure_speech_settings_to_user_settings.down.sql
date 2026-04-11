ALTER TABLE user_settings
  DROP COLUMN IF EXISTS azure_speech_region,
  DROP COLUMN IF EXISTS azure_speech_api_key_last4,
  DROP COLUMN IF EXISTS azure_speech_api_key_enc;
