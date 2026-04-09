ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS together_api_key_enc TEXT,
  ADD COLUMN IF NOT EXISTS together_api_key_last4 TEXT;
