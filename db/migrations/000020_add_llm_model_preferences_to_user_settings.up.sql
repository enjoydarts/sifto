ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS anthropic_facts_model text,
ADD COLUMN IF NOT EXISTS anthropic_summary_model text,
ADD COLUMN IF NOT EXISTS anthropic_digest_model text,
ADD COLUMN IF NOT EXISTS anthropic_source_suggestion_model text,
ADD COLUMN IF NOT EXISTS openai_embedding_model text;
