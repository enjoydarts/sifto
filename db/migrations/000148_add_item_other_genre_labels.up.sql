ALTER TABLE item_summaries
ADD COLUMN other_genre_label text;

ALTER TABLE items
ADD COLUMN user_other_genre_label text;
