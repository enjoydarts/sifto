ALTER TABLE llm_usage_logs
  DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
  ADD CONSTRAINT llm_usage_logs_purpose_check
  CHECK (purpose IN (
    'facts',
    'facts_localization',
    'facts_check',
    'summary',
    'digest',
    'embedding',
    'source_suggestion',
    'digest_cluster_draft',
    'ask',
    'faithfulness_check',
    'briefing_navigator',
    'item_navigator',
    'source_navigator',
    'ask_navigator',
    'audio_briefing_script'
  ));

CREATE TABLE IF NOT EXISTS tts_usage_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  job_id UUID REFERENCES audio_briefing_jobs(id) ON DELETE SET NULL,
  chunk_id UUID REFERENCES audio_briefing_script_chunks(id) ON DELETE SET NULL,
  provider TEXT NOT NULL,
  voice_model TEXT NOT NULL,
  voice_style TEXT NOT NULL,
  characters INTEGER NOT NULL DEFAULT 0,
  request_count INTEGER NOT NULL DEFAULT 1,
  duration_sec INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tts_usage_logs_user_created_at
  ON tts_usage_logs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tts_usage_logs_job_id
  ON tts_usage_logs (job_id);
