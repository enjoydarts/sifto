ALTER TABLE llm_usage_logs
  DROP CONSTRAINT IF EXISTS llm_usage_logs_purpose_check;

ALTER TABLE llm_usage_logs
  ADD CONSTRAINT llm_usage_logs_purpose_check
  CHECK (purpose IN (
    'facts',
    'facts_localization',
    'facts_check',
    'facts_check_precheck',
    'summary',
    'digest',
    'embedding',
    'source_suggestion',
    'digest_cluster_draft',
    'ask',
    'faithfulness_check',
    'faithfulness_check_precheck',
    'briefing_navigator',
    'item_navigator',
    'source_navigator',
    'ask_navigator',
    'audio_briefing_script',
    'ai_navigator_brief',
    'fish_preprocess',
    'gemini_tts_preprocess',
    'elevenlabs_tts_preprocess',
    'xai_tts_preprocess',
    'azure_speech_tts_preprocess'
  ));

INSERT INTO llm_usage_logs (
  idempotency_key,
  user_id,
  source_id,
  item_id,
  provider,
  model,
  requested_model,
  resolved_model,
  pricing_source,
  purpose,
  input_tokens,
  output_tokens,
  estimated_cost_usd,
  created_at
)
SELECT
  encode(digest(
    'purpose=' || CASE evaluation.kind
      WHEN 'facts' THEN 'facts_check_precheck'
      ELSE 'faithfulness_check_precheck'
    END ||
    '|provider=jev|model=' || evaluation.model ||
    '|u=' || source.user_id::text ||
    '|s=' || item.source_id::text ||
    '|i=' || evaluation.item_id::text ||
    '|d=|pk=|ps=|pvid=|pvn=0|in=' || evaluation.input_tokens::text ||
    '|out=' || evaluation.output_tokens::text ||
    '|cw=0|cr=0',
    'sha256'
  ), 'hex'),
  source.user_id,
  item.source_id,
  evaluation.item_id,
  'jev',
  evaluation.model,
  evaluation.requested_model,
  evaluation.model,
  'typesafe_jev_2026_09',
  CASE evaluation.kind
    WHEN 'facts' THEN 'facts_check_precheck'
    ELSE 'faithfulness_check_precheck'
  END,
  evaluation.input_tokens,
  evaluation.output_tokens,
  evaluation.estimated_cost_usd,
  evaluation.created_at
FROM item_quality_evaluations evaluation
JOIN items item ON item.id = evaluation.item_id
JOIN sources source ON source.id = item.source_id
WHERE evaluation.provider = 'jev'
  AND (
    evaluation.input_tokens > 0
    OR evaluation.output_tokens > 0
    OR evaluation.estimated_cost_usd > 0
  )
ON CONFLICT (idempotency_key) DO NOTHING;
