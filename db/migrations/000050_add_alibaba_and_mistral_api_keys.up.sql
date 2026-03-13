ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS alibaba_api_key_enc TEXT,
  ADD COLUMN IF NOT EXISTS alibaba_api_key_last4 TEXT,
  ADD COLUMN IF NOT EXISTS mistral_api_key_enc TEXT,
  ADD COLUMN IF NOT EXISTS mistral_api_key_last4 TEXT;
