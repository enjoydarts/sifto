DROP TABLE IF EXISTS siliconflow_model_snapshots;
DROP TABLE IF EXISTS siliconflow_model_sync_runs;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS siliconflow_api_key_last4,
  DROP COLUMN IF EXISTS siliconflow_api_key_enc;
