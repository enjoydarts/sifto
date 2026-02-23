ALTER TABLE items DROP CONSTRAINT IF EXISTS items_url_key;
ALTER TABLE items ADD CONSTRAINT items_source_id_url_key UNIQUE (source_id, url);

