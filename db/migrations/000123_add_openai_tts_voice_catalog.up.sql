CREATE TABLE openai_tts_voice_sync_runs (
    id bigserial PRIMARY KEY,
    status text NOT NULL DEFAULT 'running',
    trigger_type text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    last_progress_at timestamptz,
    fetched_count integer NOT NULL DEFAULT 0,
    saved_count integer NOT NULL DEFAULT 0,
    error_message text
);

CREATE TABLE openai_tts_voice_snapshots (
    id bigserial PRIMARY KEY,
    sync_run_id bigint NOT NULL REFERENCES openai_tts_voice_sync_runs(id) ON DELETE CASCADE,
    voice_id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    language text NOT NULL DEFAULT '',
    preview_url text NOT NULL DEFAULT '',
    metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    fetched_at timestamptz NOT NULL,
    UNIQUE (sync_run_id, voice_id)
);
