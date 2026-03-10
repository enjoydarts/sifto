ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS facts_check_model text;

CREATE TABLE IF NOT EXISTS item_facts_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id UUID UNIQUE NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  final_result TEXT NOT NULL CHECK (final_result IN ('pass', 'warn', 'fail')),
  retry_count INTEGER NOT NULL DEFAULT 0,
  short_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_item_facts_checks_item_id
ON item_facts_checks (item_id);

ALTER TABLE llm_usage_logs
  DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
  ADD CONSTRAINT llm_usage_logs_purpose_check
  CHECK (purpose IN ('facts', 'facts_check', 'summary', 'digest', 'embedding', 'source_suggestion', 'digest_cluster_draft', 'ask', 'faithfulness_check'));
