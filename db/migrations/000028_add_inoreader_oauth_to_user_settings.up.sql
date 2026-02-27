ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS inoreader_access_token_enc TEXT,
  ADD COLUMN IF NOT EXISTS inoreader_refresh_token_enc TEXT,
  ADD COLUMN IF NOT EXISTS inoreader_token_expires_at TIMESTAMPTZ;

