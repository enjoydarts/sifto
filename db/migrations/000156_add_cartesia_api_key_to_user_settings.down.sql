ALTER TABLE user_settings
    DROP COLUMN IF EXISTS cartesia_api_key_last4,
    DROP COLUMN IF EXISTS cartesia_api_key_enc;
