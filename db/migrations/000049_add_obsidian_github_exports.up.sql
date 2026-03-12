CREATE TABLE user_obsidian_exports (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  github_installation_id BIGINT,
  github_repo_owner TEXT,
  github_repo_name TEXT,
  github_repo_branch TEXT NOT NULL DEFAULT 'main',
  vault_root_path TEXT,
  keyword_link_mode TEXT NOT NULL DEFAULT 'topics_only',
  last_run_at TIMESTAMPTZ,
  last_success_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_obsidian_exports_enabled
  ON user_obsidian_exports (enabled, updated_at DESC);

CREATE TABLE item_exports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  target TEXT NOT NULL,
  github_path TEXT,
  github_sha TEXT,
  content_hash TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  exported_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, item_id, target)
);

CREATE INDEX idx_item_exports_user_target_status
  ON item_exports (user_id, target, status, updated_at DESC);
