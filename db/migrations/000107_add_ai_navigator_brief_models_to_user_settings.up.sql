ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS ai_navigator_brief_model TEXT,
ADD COLUMN IF NOT EXISTS ai_navigator_brief_fallback_model TEXT;
