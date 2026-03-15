CREATE TABLE review_queue (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    source_signal text NOT NULL,
    review_stage text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    review_due_at timestamptz NOT NULL,
    last_surfaced_at timestamptz NULL,
    completed_at timestamptz NULL,
    snooze_count int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, item_id, review_stage, status)
);

CREATE INDEX review_queue_user_status_due_idx
    ON review_queue (user_id, status, review_due_at);
