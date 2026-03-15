CREATE TABLE ask_insights (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    query text NOT NULL DEFAULT '',
    goal_id uuid NULL REFERENCES reading_goals(id) ON DELETE SET NULL,
    tags text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ask_insights_user_created_idx
    ON ask_insights (user_id, created_at DESC);

CREATE TABLE ask_insight_items (
    insight_id uuid NOT NULL REFERENCES ask_insights(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position int NOT NULL DEFAULT 0,
    PRIMARY KEY (insight_id, item_id)
);
