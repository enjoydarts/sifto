ALTER TABLE digests
  DROP COLUMN IF EXISTS cluster_draft_retry_count,
  DROP COLUMN IF EXISTS digest_retry_count;
