UPDATE item_summaries
SET genre = 'ai'
WHERE LOWER(BTRIM(COALESCE(genre, ''))) = 'agent';

UPDATE items
SET user_genre = 'ai'
WHERE LOWER(BTRIM(COALESCE(user_genre, ''))) = 'agent';
