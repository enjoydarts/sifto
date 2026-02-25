ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS anthropic_digest_cluster_model text;
