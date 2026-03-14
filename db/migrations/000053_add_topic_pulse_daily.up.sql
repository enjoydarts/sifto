CREATE TABLE IF NOT EXISTS topic_pulse_daily (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  day_jst DATE NOT NULL,
  topic_key TEXT NOT NULL,
  count INTEGER NOT NULL,
  max_score DOUBLE PRECISION,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, day_jst, topic_key)
);

CREATE INDEX IF NOT EXISTS idx_topic_pulse_daily_user_day
  ON topic_pulse_daily (user_id, day_jst DESC);

CREATE INDEX IF NOT EXISTS idx_topic_pulse_daily_user_topic_day
  ON topic_pulse_daily (user_id, topic_key, day_jst DESC);
