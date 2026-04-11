ALTER TABLE user_settings
  ADD COLUMN azure_speech_api_key_enc TEXT,
  ADD COLUMN azure_speech_api_key_last4 TEXT,
  ADD COLUMN azure_speech_region TEXT;
