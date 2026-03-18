ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS facts_fallback_model text,
  ADD COLUMN IF NOT EXISTS summary_fallback_model text;
