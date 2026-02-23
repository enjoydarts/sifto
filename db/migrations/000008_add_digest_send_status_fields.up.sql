ALTER TABLE digests
  ADD COLUMN IF NOT EXISTS send_status TEXT,
  ADD COLUMN IF NOT EXISTS send_error TEXT,
  ADD COLUMN IF NOT EXISTS send_tried_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_digests_send_status
  ON digests (send_status);
