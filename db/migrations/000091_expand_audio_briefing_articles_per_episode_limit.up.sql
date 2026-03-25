ALTER TABLE audio_briefing_settings
  DROP CONSTRAINT IF EXISTS audio_briefing_settings_articles_per_episode_check;

ALTER TABLE audio_briefing_settings
  ADD CONSTRAINT audio_briefing_settings_articles_per_episode_check
  CHECK (articles_per_episode BETWEEN 1 AND 30);
