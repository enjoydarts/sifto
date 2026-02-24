ALTER TABLE user_settings
  ADD COLUMN reading_plan_window TEXT NOT NULL DEFAULT '24h',
  ADD COLUMN reading_plan_size INTEGER NOT NULL DEFAULT 15,
  ADD COLUMN reading_plan_diversify_topics BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN reading_plan_exclude_read BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE user_settings
  ADD CONSTRAINT user_settings_reading_plan_window_check
    CHECK (reading_plan_window IN ('24h', 'today_jst', '7d'));

ALTER TABLE user_settings
  ADD CONSTRAINT user_settings_reading_plan_size_check
    CHECK (reading_plan_size >= 1 AND reading_plan_size <= 100);
