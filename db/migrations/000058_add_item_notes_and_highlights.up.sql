CREATE TABLE item_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    content text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, item_id)
);

CREATE INDEX item_notes_user_item_idx ON item_notes (user_id, item_id);

CREATE TABLE item_highlights (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    item_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quote_text text NOT NULL,
    anchor_text text NOT NULL DEFAULT '',
    section text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX item_highlights_user_item_idx ON item_highlights (user_id, item_id, created_at DESC);
