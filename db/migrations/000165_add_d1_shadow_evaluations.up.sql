ALTER TABLE user_settings
  ADD COLUMN d1_api_key_enc text,
  ADD COLUMN d1_api_key_last4 text;

ALTER TABLE item_quality_evaluations
  DROP CONSTRAINT item_quality_evaluations_item_id_kind_attempt_index_key,
  ADD CONSTRAINT item_quality_evaluations_item_kind_attempt_provider_key UNIQUE (item_id, kind, attempt_index, provider);
