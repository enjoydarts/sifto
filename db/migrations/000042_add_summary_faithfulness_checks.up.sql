ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS faithfulness_check_model text;

CREATE TABLE IF NOT EXISTS summary_faithfulness_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id UUID UNIQUE NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  final_result TEXT NOT NULL CHECK (final_result IN ('pass', 'warn', 'fail')),
  retry_count INTEGER NOT NULL DEFAULT 0,
  short_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_summary_faithfulness_checks_item_id
ON summary_faithfulness_checks (item_id);
