DROP TABLE IF EXISTS deepinfra_model_snapshots;
DROP TABLE IF EXISTS deepinfra_model_sync_runs;

ALTER TABLE provider_model_change_events
  DROP CONSTRAINT IF EXISTS provider_model_change_events_change_type_check;

ALTER TABLE provider_model_change_events
  ADD CONSTRAINT provider_model_change_events_change_type_check
  CHECK (change_type IN ('added', 'constrained', 'availability_changed', 'gated_changed', 'removed'));

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS deepinfra_api_key_last4,
  DROP COLUMN IF EXISTS deepinfra_api_key_enc;
