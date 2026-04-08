ALTER TABLE llm_usage_logs
  DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
  ADD CONSTRAINT llm_usage_logs_purpose_check
  CHECK (purpose IN (
    'facts',
    'facts_localization',
    'facts_check',
    'summary',
    'digest',
    'embedding',
    'source_suggestion',
    'digest_cluster_draft',
    'ask',
    'faithfulness_check',
    'briefing_navigator',
    'item_navigator',
    'source_navigator',
    'ask_navigator',
    'audio_briefing_script',
    'ai_navigator_brief',
    'fish_preprocess',
    'gemini_tts_preprocess',
    'elevenlabs_tts_preprocess'
  ));
