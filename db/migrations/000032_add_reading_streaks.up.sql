CREATE TABLE IF NOT EXISTS reading_streaks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  streak_date DATE NOT NULL,
  read_count INTEGER NOT NULL DEFAULT 0,
  streak_days INTEGER NOT NULL DEFAULT 0,
  is_completed BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, streak_date)
);

CREATE INDEX IF NOT EXISTS idx_reading_streaks_user_date
  ON reading_streaks(user_id, streak_date DESC);

