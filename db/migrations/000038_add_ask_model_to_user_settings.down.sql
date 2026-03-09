ALTER TABLE user_settings
ALTER COLUMN anthropic_ask_model DROP DEFAULT;

ALTER TABLE user_settings
DROP COLUMN IF EXISTS anthropic_ask_model;
