ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS ai_navigator_brief_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS ai_navigator_briefs (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slot TEXT NOT NULL,
  status TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  intro TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  persona TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  source_window_start TIMESTAMPTZ,
  source_window_end TIMESTAMPTZ,
  generated_at TIMESTAMPTZ,
  notification_sent_at TIMESTAMPTZ,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_navigator_briefs_user_generated_at
  ON ai_navigator_briefs (user_id, generated_at DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_navigator_briefs_user_slot_generated_at
  ON ai_navigator_briefs (user_id, slot, generated_at DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_navigator_briefs_user_status_generated_at
  ON ai_navigator_briefs (user_id, status, generated_at DESC, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_navigator_brief_items (
  id UUID PRIMARY KEY,
  brief_id UUID NOT NULL REFERENCES ai_navigator_briefs(id) ON DELETE CASCADE,
  rank INTEGER NOT NULL,
  item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  title_snapshot TEXT NOT NULL DEFAULT '',
  translated_title_snapshot TEXT NOT NULL DEFAULT '',
  source_title_snapshot TEXT NOT NULL DEFAULT '',
  comment TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (brief_id, rank)
);

CREATE INDEX IF NOT EXISTS idx_ai_navigator_brief_items_brief_rank
  ON ai_navigator_brief_items (brief_id, rank ASC);
