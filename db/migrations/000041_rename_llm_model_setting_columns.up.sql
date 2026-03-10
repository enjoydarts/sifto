ALTER TABLE user_settings
RENAME COLUMN anthropic_facts_model TO facts_model;

ALTER TABLE user_settings
RENAME COLUMN anthropic_summary_model TO summary_model;

ALTER TABLE user_settings
RENAME COLUMN anthropic_digest_cluster_model TO digest_cluster_model;

ALTER TABLE user_settings
RENAME COLUMN anthropic_digest_model TO digest_model;

ALTER TABLE user_settings
RENAME COLUMN anthropic_ask_model TO ask_model;

ALTER TABLE user_settings
RENAME COLUMN anthropic_source_suggestion_model TO source_suggestion_model;

ALTER TABLE user_settings
RENAME COLUMN openai_embedding_model TO embedding_model;
