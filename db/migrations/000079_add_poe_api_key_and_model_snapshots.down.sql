DROP TABLE IF EXISTS poe_model_snapshots;
DROP TABLE IF EXISTS poe_model_sync_runs;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS poe_api_key_last4,
  DROP COLUMN IF EXISTS poe_api_key_enc;
