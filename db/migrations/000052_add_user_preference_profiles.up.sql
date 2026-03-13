CREATE TABLE user_preference_profiles (
  user_id           UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  learned_weights   JSONB NOT NULL DEFAULT '{}',
  topic_interests   JSONB NOT NULL DEFAULT '{}',
  pref_embedding    DOUBLE PRECISION[],
  source_affinities JSONB NOT NULL DEFAULT '{}',
  feedback_count    INTEGER NOT NULL DEFAULT 0,
  read_count        INTEGER NOT NULL DEFAULT 0,
  computed_at       TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
