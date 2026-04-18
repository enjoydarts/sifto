ALTER TABLE user_settings
  DROP COLUMN IF EXISTS xiaomi_mimo_token_plan_api_key_last4,
  DROP COLUMN IF EXISTS xiaomi_mimo_token_plan_api_key_enc;
