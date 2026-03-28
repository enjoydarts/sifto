ALTER TABLE user_settings
  DROP COLUMN IF EXISTS summary_secondary_rate_percent,
  DROP COLUMN IF EXISTS summary_secondary_model,
  DROP COLUMN IF EXISTS facts_secondary_rate_percent,
  DROP COLUMN IF EXISTS facts_secondary_model;
