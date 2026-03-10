ALTER TABLE user_settings
RENAME COLUMN facts_model TO anthropic_facts_model;

ALTER TABLE user_settings
RENAME COLUMN summary_model TO anthropic_summary_model;

ALTER TABLE user_settings
RENAME COLUMN digest_cluster_model TO anthropic_digest_cluster_model;

ALTER TABLE user_settings
RENAME COLUMN digest_model TO anthropic_digest_model;

ALTER TABLE user_settings
RENAME COLUMN ask_model TO anthropic_ask_model;

ALTER TABLE user_settings
RENAME COLUMN source_suggestion_model TO anthropic_source_suggestion_model;

ALTER TABLE user_settings
RENAME COLUMN embedding_model TO openai_embedding_model;
