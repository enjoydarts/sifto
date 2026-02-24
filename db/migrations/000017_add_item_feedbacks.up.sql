CREATE TABLE IF NOT EXISTS item_feedbacks (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  rating SMALLINT NOT NULL DEFAULT 0 CHECK (rating IN (-1, 0, 1)),
  is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_item_feedbacks_user_updated_at
  ON item_feedbacks (user_id, updated_at DESC);
