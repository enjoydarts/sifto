UPDATE item_quality_evaluations
SET escalation_reason = 'critical_dimension_low'
WHERE escalation_reason IN (
  'unsupported_claim_risk', 'contradiction_risk', 'entity_numeric_mismatch_risk'
);

ALTER TABLE item_quality_evaluations
  DROP CONSTRAINT IF EXISTS item_quality_evaluations_escalation_reason_check;

ALTER TABLE item_quality_evaluations
  ADD CONSTRAINT item_quality_evaluations_escalation_reason_check
  CHECK (escalation_reason IS NULL OR escalation_reason IN (
    'low_score', 'low_dimension_score', 'low_confidence', 'critical_dimension_low',
    'timeout', 'http_error', 'schema_error', 'configuration_error'
  ));

ALTER TABLE item_quality_evaluations
  DROP COLUMN IF EXISTS signal_thresholds_json,
  DROP COLUMN IF EXISTS signals_json;
