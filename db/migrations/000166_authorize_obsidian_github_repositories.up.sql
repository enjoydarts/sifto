ALTER TABLE user_obsidian_exports
ADD COLUMN github_authorized_repositories text[] NOT NULL DEFAULT '{}';
