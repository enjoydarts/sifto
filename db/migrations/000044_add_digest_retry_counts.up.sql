ALTER TABLE digests
  ADD COLUMN IF NOT EXISTS digest_retry_count integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cluster_draft_retry_count integer NOT NULL DEFAULT 0;
