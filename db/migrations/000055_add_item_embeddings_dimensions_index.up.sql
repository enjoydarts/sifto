CREATE INDEX IF NOT EXISTS idx_item_embeddings_dimensions_item
  ON item_embeddings (dimensions, item_id);
