ALTER TABLE notification_priority_rules
    ADD COLUMN immediate_enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN briefing_enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN review_enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN goal_match_enabled boolean NOT NULL DEFAULT true;
