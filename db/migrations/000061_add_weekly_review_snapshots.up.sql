CREATE TABLE weekly_review_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    week_start date NOT NULL,
    week_end date NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, week_start)
);

CREATE INDEX weekly_review_snapshots_user_created_idx
    ON weekly_review_snapshots (user_id, created_at DESC);
