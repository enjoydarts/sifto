ALTER TABLE items
DROP COLUMN IF EXISTS user_other_genre_label;

ALTER TABLE item_summaries
DROP COLUMN IF EXISTS other_genre_label;
