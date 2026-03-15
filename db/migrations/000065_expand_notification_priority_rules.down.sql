ALTER TABLE notification_priority_rules
    DROP COLUMN IF EXISTS goal_match_enabled,
    DROP COLUMN IF EXISTS review_enabled,
    DROP COLUMN IF EXISTS briefing_enabled,
    DROP COLUMN IF EXISTS immediate_enabled;
