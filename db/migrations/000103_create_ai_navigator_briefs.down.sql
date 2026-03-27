DROP TABLE IF EXISTS ai_navigator_brief_items;
DROP TABLE IF EXISTS ai_navigator_briefs;

ALTER TABLE user_settings
  DROP COLUMN IF EXISTS ai_navigator_brief_enabled;
