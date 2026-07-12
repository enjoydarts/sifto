ALTER TABLE sources
  DROP COLUMN IF EXISTS feed_last_modified,
  DROP COLUMN IF EXISTS feed_etag;
