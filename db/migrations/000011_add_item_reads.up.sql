CREATE TABLE IF NOT EXISTS item_reads (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_item_reads_user_read_at
    ON item_reads (user_id, read_at DESC);

