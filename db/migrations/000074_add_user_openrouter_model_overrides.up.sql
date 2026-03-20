CREATE TABLE IF NOT EXISTS user_openrouter_model_overrides (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  model_id TEXT NOT NULL,
  allow_structured_output BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, model_id)
);

CREATE INDEX IF NOT EXISTS user_openrouter_model_overrides_user_idx
  ON user_openrouter_model_overrides(user_id);
