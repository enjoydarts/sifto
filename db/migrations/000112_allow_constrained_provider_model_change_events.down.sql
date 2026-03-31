ALTER TABLE provider_model_change_events
  DROP CONSTRAINT IF EXISTS provider_model_change_events_change_type_check;

ALTER TABLE provider_model_change_events
  ADD CONSTRAINT provider_model_change_events_change_type_check
  CHECK (change_type IN ('added', 'removed'));
