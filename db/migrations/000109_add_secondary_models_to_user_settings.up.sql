ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS facts_secondary_model TEXT,
  ADD COLUMN IF NOT EXISTS facts_secondary_rate_percent INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS summary_secondary_model TEXT,
  ADD COLUMN IF NOT EXISTS summary_secondary_rate_percent INT NOT NULL DEFAULT 0;
