CREATE TABLE llm_value_metrics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    window_start date NOT NULL,
    window_end date NOT NULL,
    purpose text NOT NULL,
    provider text NOT NULL,
    model text NOT NULL,
    metrics jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX llm_value_metrics_window_idx
    ON llm_value_metrics (user_id, window_start, purpose, provider, model);

CREATE INDEX llm_value_metrics_created_idx
    ON llm_value_metrics (user_id, created_at DESC);
