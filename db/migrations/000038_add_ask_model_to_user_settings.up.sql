ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS anthropic_ask_model text;

ALTER TABLE user_settings
ALTER COLUMN anthropic_ask_model SET DEFAULT 'gemini-2.5-flash';

UPDATE user_settings
SET anthropic_ask_model = COALESCE(anthropic_ask_model, anthropic_digest_model, anthropic_summary_model, 'gemini-2.5-flash');
