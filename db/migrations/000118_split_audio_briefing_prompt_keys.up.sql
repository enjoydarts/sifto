UPDATE prompt_templates
SET
  key = 'audio_briefing_script.single',
  description = 'Audio briefing single-speaker script prompt management entry',
  updated_at = NOW()
WHERE key = 'audio_briefing_script.default';

INSERT INTO prompt_templates (key, purpose, description, status)
VALUES
  ('audio_briefing_script.duo', 'audio_briefing_script', 'Audio briefing duo script prompt management entry', 'active')
ON CONFLICT (key) DO NOTHING;
