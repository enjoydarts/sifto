ALTER TABLE item_summaries
ADD COLUMN genre text;

ALTER TABLE items
ADD COLUMN user_genre text;
