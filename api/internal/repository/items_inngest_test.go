package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testItemInngestRepoDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	lockItemInngestRepoTestDB(t, pool)

	if _, err := pool.Exec(context.Background(), `
		ALTER TABLE items ADD COLUMN IF NOT EXISTS user_genre text;
		ALTER TABLE items ADD COLUMN IF NOT EXISTS user_other_genre_label text;
		ALTER TABLE item_summaries ADD COLUMN IF NOT EXISTS genre text;
		ALTER TABLE item_summaries ADD COLUMN IF NOT EXISTS other_genre_label text;

		DELETE FROM item_summaries WHERE item_id = '00000000-0000-4000-8000-000000000241';
		DELETE FROM items WHERE id = '00000000-0000-4000-8000-000000000241';
		DELETE FROM sources WHERE id = '00000000-0000-4000-8000-000000000231';
		DELETE FROM users WHERE id = '00000000-0000-4000-8000-000000000221';

		INSERT INTO users (id, email, name)
		VALUES ('00000000-0000-4000-8000-000000000221', 'item-inngest-repo@example.com', 'Item Inngest Repo');

		INSERT INTO sources (id, user_id, url, type, title)
		VALUES ('00000000-0000-4000-8000-000000000231', '00000000-0000-4000-8000-000000000221', 'https://example.com/feed', 'manual', 'Example Feed');

		INSERT INTO items (id, source_id, url, title, status, content_text)
		VALUES ('00000000-0000-4000-8000-000000000241', '00000000-0000-4000-8000-000000000231', 'https://example.com/items/241', 'Example Item', 'facts_extracted', 'body');
	`); err != nil {
		t.Fatalf("reset item inngest repo tables: %v", err)
	}

	return pool
}

func lockItemInngestRepoTestDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const key int64 = 74231011
	if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_lock($1)`, key); err != nil {
		t.Fatalf("pg_advisory_lock() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key); err != nil {
			t.Fatalf("pg_advisory_unlock() error = %v", err)
		}
	})
}

func testItemInngestRepoStringPtr(v string) *string { return &v }

func TestItemInngestRepoInsertSummaryPersistsOtherGenreLabel(t *testing.T) {
	ctx := context.Background()
	pool := testItemInngestRepoDB(t)
	repo := NewItemInngestRepo(pool)

	if err := repo.InsertSummary(
		ctx,
		"00000000-0000-4000-8000-000000000241",
		"summary",
		[]string{"ai"},
		testItemInngestRepoStringPtr("other"),
		testItemInngestRepoStringPtr("Observability"),
		"Translated",
		0.9,
		nil,
		"",
		"",
	); err != nil {
		t.Fatalf("InsertSummary() error = %v", err)
	}

	var genre *string
	var otherLabel *string
	if err := pool.QueryRow(ctx, `
		SELECT genre, other_genre_label
		FROM item_summaries
		WHERE item_id = $1
	`, "00000000-0000-4000-8000-000000000241").Scan(&genre, &otherLabel); err != nil {
		t.Fatalf("query item_summaries: %v", err)
	}

	if genre == nil || *genre != "other" {
		t.Fatalf("genre = %#v, want other", genre)
	}
	if otherLabel == nil || *otherLabel != "Observability" {
		t.Fatalf("other_genre_label = %#v, want Observability", otherLabel)
	}
}
