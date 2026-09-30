DELETE FROM item_quality_evaluations WHERE provider = 'd1';
ALTER TABLE item_quality_evaluations
  DROP CONSTRAINT item_quality_evaluations_item_kind_attempt_provider_key,
  ADD CONSTRAINT item_quality_evaluations_item_id_kind_attempt_index_key UNIQUE (item_id, kind, attempt_index);
ALTER TABLE user_settings DROP COLUMN d1_api_key_enc, DROP COLUMN d1_api_key_last4;
