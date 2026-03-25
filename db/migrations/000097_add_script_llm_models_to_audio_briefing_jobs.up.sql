ALTER TABLE audio_briefing_jobs
  ADD COLUMN IF NOT EXISTS script_llm_models TEXT;
