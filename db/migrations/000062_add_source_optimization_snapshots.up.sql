CREATE TABLE source_optimization_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    source_id uuid NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    window_start date NOT NULL,
    window_end date NOT NULL,
    metrics jsonb NOT NULL,
    recommendation text NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX source_optimization_snapshots_user_source_idx
    ON source_optimization_snapshots (user_id, source_id, created_at DESC);
