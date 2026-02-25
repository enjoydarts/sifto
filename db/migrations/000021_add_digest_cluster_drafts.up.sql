CREATE TABLE digest_cluster_drafts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  digest_id UUID NOT NULL REFERENCES digests(id) ON DELETE CASCADE,
  cluster_key TEXT NOT NULL,
  cluster_label TEXT NOT NULL,
  rank INT NOT NULL,
  item_count INT NOT NULL DEFAULT 0,
  topics TEXT[] NOT NULL DEFAULT '{}'::text[],
  max_score DOUBLE PRECISION NULL,
  draft_summary TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (digest_id, cluster_key)
);

CREATE INDEX idx_digest_cluster_drafts_digest_rank ON digest_cluster_drafts (digest_id, rank);
