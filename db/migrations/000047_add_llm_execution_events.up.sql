CREATE TABLE IF NOT EXISTS llm_execution_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  source_id UUID REFERENCES sources(id) ON DELETE SET NULL,
  item_id UUID REFERENCES items(id) ON DELETE SET NULL,
  digest_id UUID REFERENCES digests(id) ON DELETE SET NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  purpose TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('success', 'failure')),
  attempt_index INTEGER NOT NULL DEFAULT 0,
  empty_response BOOLEAN NOT NULL DEFAULT FALSE,
  error_kind TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_execution_events_user_created_at
  ON llm_execution_events (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_execution_events_user_purpose_created_at
  ON llm_execution_events (user_id, purpose, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_execution_events_user_model_created_at
  ON llm_execution_events (user_id, provider, model, created_at DESC);
