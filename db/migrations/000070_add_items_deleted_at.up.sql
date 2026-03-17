ALTER TABLE items
    ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX idx_items_visible_source_created_at
    ON items (source_id, created_at DESC)
    WHERE deleted_at IS NULL;
