ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS navigator_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS navigator_persona TEXT NOT NULL DEFAULT 'editor',
  ADD COLUMN IF NOT EXISTS navigator_model TEXT,
  ADD COLUMN IF NOT EXISTS navigator_fallback_model TEXT;
