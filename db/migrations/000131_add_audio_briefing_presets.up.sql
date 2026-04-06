CREATE TABLE IF NOT EXISTS audio_briefing_presets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  default_persona_mode TEXT NOT NULL DEFAULT 'fixed',
  default_persona TEXT NOT NULL DEFAULT 'editor',
  conversation_mode TEXT NOT NULL DEFAULT 'single',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_presets_user_id_updated_at
  ON audio_briefing_presets (user_id, updated_at DESC, created_at DESC, name ASC);

CREATE TABLE IF NOT EXISTS audio_briefing_preset_voices (
  preset_id UUID NOT NULL REFERENCES audio_briefing_presets(id) ON DELETE CASCADE,
  persona TEXT NOT NULL,
  tts_provider TEXT NOT NULL,
  tts_model TEXT NOT NULL DEFAULT '',
  voice_model TEXT NOT NULL DEFAULT '',
  voice_style TEXT NOT NULL DEFAULT '',
  provider_voice_label TEXT,
  provider_voice_description TEXT,
  speech_rate DOUBLE PRECISION NOT NULL DEFAULT 1,
  emotional_intensity DOUBLE PRECISION NOT NULL DEFAULT 1,
  tempo_dynamics DOUBLE PRECISION NOT NULL DEFAULT 1,
  line_break_silence_seconds DOUBLE PRECISION NOT NULL DEFAULT 0.4,
  pitch DOUBLE PRECISION NOT NULL DEFAULT 0,
  volume_gain DOUBLE PRECISION NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (preset_id, persona)
);

CREATE INDEX IF NOT EXISTS idx_audio_briefing_preset_voices_preset_id_persona
  ON audio_briefing_preset_voices (preset_id, persona);
