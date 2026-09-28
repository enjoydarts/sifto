ALTER TABLE item_quality_evaluations
  ADD COLUMN signals_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN signal_thresholds_json jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE item_quality_evaluations
  DROP CONSTRAINT IF EXISTS item_quality_evaluations_escalation_reason_check;

ALTER TABLE item_quality_evaluations
  ADD CONSTRAINT item_quality_evaluations_escalation_reason_check
  CHECK (escalation_reason IS NULL OR escalation_reason IN (
    'low_score', 'low_dimension_score', 'low_confidence', 'critical_dimension_low',
    'unsupported_claim_risk', 'contradiction_risk', 'entity_numeric_mismatch_risk',
    'timeout', 'http_error', 'schema_error', 'configuration_error'
  ));
