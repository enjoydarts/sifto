ALTER TABLE user_settings
  DROP COLUMN IF EXISTS mistral_api_key_last4,
  DROP COLUMN IF EXISTS mistral_api_key_enc,
  DROP COLUMN IF EXISTS alibaba_api_key_last4,
  DROP COLUMN IF EXISTS alibaba_api_key_enc;
