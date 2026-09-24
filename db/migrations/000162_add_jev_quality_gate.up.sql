ALTER TABLE user_settings
  ADD COLUMN jev_api_key_enc text,
  ADD COLUMN jev_api_key_last4 text;

CREATE TABLE item_quality_evaluations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('facts', 'faithfulness')),
  attempt_index integer NOT NULL CHECK (attempt_index >= 0),
  provider text NOT NULL DEFAULT 'jev',
  requested_model text NOT NULL,
  model text NOT NULL,
  dimensions_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  aggregate_score double precision NOT NULL DEFAULT 0,
  minimum_score double precision NOT NULL DEFAULT 0,
  minimum_confidence double precision NOT NULL DEFAULT 0,
  quality_threshold double precision NOT NULL,
  confidence_threshold double precision NOT NULL,
  gate_policy_version text NOT NULL,
  decision text NOT NULL CHECK (decision IN ('accepted', 'escalated', 'error')),
  escalation_reason text CHECK (escalation_reason IS NULL OR escalation_reason IN (
    'low_score', 'low_dimension_score', 'low_confidence', 'critical_dimension_low',
    'timeout', 'http_error', 'schema_error', 'configuration_error'
  )),
  reason_detail text,
  input_tokens integer NOT NULL DEFAULT 0,
  output_tokens integer NOT NULL DEFAULT 0,
  estimated_cost_usd double precision NOT NULL DEFAULT 0,
  latency_ms bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (item_id, kind, attempt_index)
);

CREATE INDEX item_quality_evaluations_item_kind_created_idx
  ON item_quality_evaluations (item_id, kind, created_at DESC);
