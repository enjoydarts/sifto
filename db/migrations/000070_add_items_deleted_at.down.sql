DROP INDEX IF EXISTS idx_items_visible_source_created_at;

ALTER TABLE items
    DROP COLUMN IF EXISTS deleted_at;
