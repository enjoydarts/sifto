CREATE TABLE notification_priority_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL UNIQUE,
    sensitivity text NOT NULL DEFAULT 'medium',
    daily_cap int NOT NULL DEFAULT 3,
    theme_weight numeric NOT NULL DEFAULT 1.0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
