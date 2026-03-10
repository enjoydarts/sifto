ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS deepseek_api_key_enc text,
ADD COLUMN IF NOT EXISTS deepseek_api_key_last4 text;
