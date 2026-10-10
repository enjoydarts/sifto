ALTER TABLE user_settings ADD COLUMN alibaba_workspace_id text;
ALTER TABLE user_settings ADD CONSTRAINT user_settings_alibaba_workspace_id_check
    CHECK (alibaba_workspace_id IS NULL OR alibaba_workspace_id ~ '^ws-[a-z0-9]{1,60}$');
