CREATE TABLE IF NOT EXISTS push_notification_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  item_id UUID NULL REFERENCES items(id) ON DELETE SET NULL,
  day_jst DATE NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  onesignal_notification_id TEXT NULL,
  recipients INT NOT NULL DEFAULT 0,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_push_notification_logs_user_kind_item
  ON push_notification_logs(user_id, kind, item_id)
  WHERE item_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_push_notification_logs_user_kind_day
  ON push_notification_logs(user_id, kind, day_jst);
