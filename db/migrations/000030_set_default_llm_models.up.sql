-- Default model policy:
-- facts: gemini-2.5-flash-lite
-- summary: gemini-2.5-flash
-- digest cluster draft: gemini-2.5-flash
-- digest compose: gemini-3.1-pro-preview
-- source suggestion: claude-sonnet-4-6 (anthropic-only flow)
-- embedding: text-embedding-3-small

ALTER TABLE user_settings
ALTER COLUMN anthropic_facts_model SET DEFAULT 'gemini-2.5-flash-lite',
ALTER COLUMN anthropic_summary_model SET DEFAULT 'gemini-2.5-flash',
ALTER COLUMN anthropic_digest_cluster_model SET DEFAULT 'gemini-2.5-flash',
ALTER COLUMN anthropic_digest_model SET DEFAULT 'gemini-3.1-pro-preview',
ALTER COLUMN anthropic_source_suggestion_model SET DEFAULT 'claude-sonnet-4-6',
ALTER COLUMN openai_embedding_model SET DEFAULT 'text-embedding-3-small';

UPDATE user_settings
SET anthropic_facts_model = COALESCE(anthropic_facts_model, 'gemini-2.5-flash-lite'),
    anthropic_summary_model = COALESCE(anthropic_summary_model, 'gemini-2.5-flash'),
    anthropic_digest_cluster_model = COALESCE(anthropic_digest_cluster_model, 'gemini-2.5-flash'),
    anthropic_digest_model = COALESCE(anthropic_digest_model, 'gemini-3.1-pro-preview'),
    anthropic_source_suggestion_model = COALESCE(anthropic_source_suggestion_model, 'claude-sonnet-4-6'),
    openai_embedding_model = COALESCE(openai_embedding_model, 'text-embedding-3-small');
