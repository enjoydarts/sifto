CREATE TABLE IF NOT EXISTS llm_usage_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  source_id UUID REFERENCES sources(id) ON DELETE SET NULL,
  item_id UUID REFERENCES items(id) ON DELETE SET NULL,
  digest_id UUID REFERENCES digests(id) ON DELETE SET NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  purpose TEXT NOT NULL CHECK (purpose IN ('facts', 'summary', 'digest')),
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  cache_creation_input_tokens INTEGER NOT NULL DEFAULT 0,
  cache_read_input_tokens INTEGER NOT NULL DEFAULT 0,
  estimated_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_created_at
  ON llm_usage_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_user_created_at
  ON llm_usage_logs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_item_id
  ON llm_usage_logs (item_id);

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_digest_id
  ON llm_usage_logs (digest_id);
