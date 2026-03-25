CREATE TABLE IF NOT EXISTS audio_briefing_settings (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  interval_hours INTEGER NOT NULL DEFAULT 6 CHECK (interval_hours IN (3, 6)),
  articles_per_episode INTEGER NOT NULL DEFAULT 5 CHECK (articles_per_episode BETWEEN 1 AND 12),
  target_duration_minutes INTEGER NOT NULL DEFAULT 20 CHECK (target_duration_minutes BETWEEN 5 AND 60),
  default_persona TEXT NOT NULL DEFAULT 'editor',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audio_briefing_persona_voices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  persona TEXT NOT NULL,
  tts_provider TEXT NOT NULL,
  voice_model TEXT NOT NULL,
  voice_style TEXT NOT NULL,
  speech_rate REAL NOT NULL DEFAULT 1.0,
  pitch REAL NOT NULL DEFAULT 0.0,
  volume_gain REAL NOT NULL DEFAULT 0.0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, persona)
);

CREATE TABLE IF NOT EXISTS audio_briefing_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slot_started_at_jst TIMESTAMPTZ NOT NULL,
  slot_key TEXT NOT NULL,
  persona TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN (
    'pending',
    'scripting',
    'scripted',
    'voicing',
    'voiced',
    'concatenating',
    'published',
    'skipped',
    'failed',
    'needs_rerun',
    'cancelled'
  )),
  source_item_count INTEGER NOT NULL DEFAULT 0,
  reused_item_count INTEGER NOT NULL DEFAULT 0,
  script_char_count INTEGER NOT NULL DEFAULT 0,
  audio_duration_sec INTEGER,
  title TEXT,
  r2_audio_object_key TEXT,
  r2_manifest_object_key TEXT,
  provider_job_id TEXT,
  idempotency_key TEXT,
  error_code TEXT,
  error_message TEXT,
  published_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, slot_key),
  UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_jobs_user_created_at
  ON audio_briefing_jobs (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_jobs_status_created_at
  ON audio_briefing_jobs (status, created_at DESC);

CREATE TABLE IF NOT EXISTS audio_briefing_job_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id UUID NOT NULL REFERENCES audio_briefing_jobs(id) ON DELETE CASCADE,
  item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  rank INTEGER NOT NULL,
  segment_title TEXT,
  summary_snapshot TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (job_id, item_id),
  UNIQUE (job_id, rank)
);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_job_items_item_id
  ON audio_briefing_job_items (item_id);

CREATE TABLE IF NOT EXISTS audio_briefing_script_chunks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id UUID NOT NULL REFERENCES audio_briefing_jobs(id) ON DELETE CASCADE,
  seq INTEGER NOT NULL,
  part_type TEXT NOT NULL CHECK (part_type IN ('opening', 'summary', 'article', 'ending')),
  text TEXT NOT NULL,
  char_count INTEGER NOT NULL,
  tts_status TEXT NOT NULL DEFAULT 'pending' CHECK (tts_status IN ('pending', 'generating', 'generated', 'failed')),
  tts_provider TEXT,
  voice_model TEXT,
  voice_style TEXT,
  r2_audio_object_key TEXT,
  duration_sec INTEGER,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (job_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_script_chunks_job_seq
  ON audio_briefing_script_chunks (job_id, seq);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_script_chunks_tts_status
  ON audio_briefing_script_chunks (tts_status, created_at DESC);

CREATE TABLE IF NOT EXISTS audio_briefing_callback_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id UUID NOT NULL REFERENCES audio_briefing_jobs(id) ON DELETE CASCADE,
  request_id TEXT NOT NULL,
  provider_job_id TEXT,
  token_hash TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (job_id, request_id),
  UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_callback_tokens_expires_at
  ON audio_briefing_callback_tokens (expires_at);
