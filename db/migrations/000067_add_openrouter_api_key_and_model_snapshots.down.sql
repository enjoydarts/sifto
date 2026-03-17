DROP TABLE IF EXISTS openrouter_model_notification_logs;
DROP TABLE IF EXISTS openrouter_model_snapshots;
DROP TABLE IF EXISTS openrouter_model_sync_runs;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS openrouter_api_key_last4,
  DROP COLUMN IF EXISTS openrouter_api_key_enc;
