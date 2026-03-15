CREATE TABLE reading_goals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    priority int NOT NULL DEFAULT 3,
    status text NOT NULL DEFAULT 'active',
    due_date date NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reading_goals_status_check CHECK (status IN ('active', 'archived')),
    CONSTRAINT reading_goals_priority_check CHECK (priority BETWEEN 1 AND 5)
);

CREATE INDEX reading_goals_user_status_priority_idx
    ON reading_goals (user_id, status, priority DESC, updated_at DESC);
