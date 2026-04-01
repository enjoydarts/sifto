DELETE FROM prompt_templates
WHERE key = 'audio_briefing_script.duo';

UPDATE prompt_templates
SET
  key = 'audio_briefing_script.default',
  description = 'Audio briefing script prompt management entry',
  updated_at = NOW()
WHERE key = 'audio_briefing_script.single';
