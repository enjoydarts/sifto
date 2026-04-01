CREATE TABLE IF NOT EXISTS prompt_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,
  purpose TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  active_version_id UUID NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prompt_template_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES prompt_templates(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  label TEXT NOT NULL DEFAULT '',
  system_instruction TEXT NOT NULL DEFAULT '',
  prompt_text TEXT NOT NULL,
  fallback_prompt_text TEXT NOT NULL DEFAULT '',
  variables_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  notes TEXT NOT NULL DEFAULT '',
  created_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
  created_by_email TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (template_id, version)
);

ALTER TABLE prompt_templates
  ADD CONSTRAINT prompt_templates_active_version_id_fkey
  FOREIGN KEY (active_version_id) REFERENCES prompt_template_versions(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS prompt_experiments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES prompt_templates(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  assignment_unit TEXT NOT NULL,
  started_at TIMESTAMPTZ NULL,
  ended_at TIMESTAMPTZ NULL,
  created_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
  created_by_email TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prompt_experiment_arms (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  experiment_id UUID NOT NULL REFERENCES prompt_experiments(id) ON DELETE CASCADE,
  version_id UUID NOT NULL REFERENCES prompt_template_versions(id) ON DELETE CASCADE,
  weight INTEGER NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prompt_admin_audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
  user_email TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  template_id UUID NULL REFERENCES prompt_templates(id) ON DELETE SET NULL,
  version_id UUID NULL REFERENCES prompt_template_versions(id) ON DELETE SET NULL,
  experiment_id UUID NULL REFERENCES prompt_experiments(id) ON DELETE SET NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE llm_usage_logs
  ADD COLUMN IF NOT EXISTS prompt_key TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS prompt_source TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS prompt_version_id UUID NULL REFERENCES prompt_template_versions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS prompt_version_number INTEGER NULL,
  ADD COLUMN IF NOT EXISTS prompt_experiment_id UUID NULL REFERENCES prompt_experiments(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS prompt_experiment_arm_id UUID NULL REFERENCES prompt_experiment_arms(id) ON DELETE SET NULL;

ALTER TABLE llm_execution_events
  ADD COLUMN IF NOT EXISTS prompt_key TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS prompt_source TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS prompt_version_id UUID NULL REFERENCES prompt_template_versions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS prompt_version_number INTEGER NULL,
  ADD COLUMN IF NOT EXISTS prompt_experiment_id UUID NULL REFERENCES prompt_experiments(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS prompt_experiment_arm_id UUID NULL REFERENCES prompt_experiment_arms(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_prompt_templates_purpose ON prompt_templates(purpose);
CREATE INDEX IF NOT EXISTS idx_prompt_template_versions_template_id ON prompt_template_versions(template_id, version DESC);
CREATE INDEX IF NOT EXISTS idx_prompt_experiments_template_id ON prompt_experiments(template_id, status);
CREATE INDEX IF NOT EXISTS idx_prompt_experiment_arms_experiment_id ON prompt_experiment_arms(experiment_id);

INSERT INTO prompt_templates (key, purpose, description, status)
VALUES
  ('summary.default', 'summary', 'Summary prompt management entry', 'active'),
  ('facts.default', 'facts', 'Facts extraction prompt management entry', 'active'),
  ('digest.default', 'digest', 'Digest composition prompt management entry', 'active'),
  ('audio_briefing_script.default', 'audio_briefing_script', 'Audio briefing script prompt management entry', 'active')
ON CONFLICT (key) DO NOTHING;
