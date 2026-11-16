package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFavoriteExportItemsReleasesRowsBeforeLoadingMetadata(t *testing.T) {
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var userID, sourceID, itemID string
	if err := pool.QueryRow(context.Background(), `INSERT INTO users(email) VALUES ('export-pool-' || gen_random_uuid()::text || '@example.com') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM item_notes WHERE user_id=$1`, userID); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID); err != nil {
			t.Error(err)
		}
	})
	if err := pool.QueryRow(context.Background(), `INSERT INTO sources(user_id,url,type) VALUES ($1,'https://example.com/export-pool','manual') RETURNING id`, userID).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `INSERT INTO items(source_id,url,status,title) VALUES ($1::uuid,'https://example.com/export-pool/' || $1::text,'summarized','Favorite') RETURNING id`, sourceID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO item_feedbacks(user_id,item_id,is_favorite) VALUES ($1,$2,true)`, []any{userID, itemID}},
		{`INSERT INTO item_summaries(item_id,summary) VALUES ($1,repeat('summary ',20000))`, []any{itemID}},
		{`INSERT INTO item_notes(user_id,item_id,content) VALUES ($1,$2,'Export note')`, []any{userID, itemID}},
	} {
		if _, err := pool.Exec(context.Background(), fixture.query, fixture.args...); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	items, err := NewItemRepo(pool).FavoriteExportItems(ctx, userID, 0, 100)
	if ctx.Err() != nil {
		t.Fatalf("export blocked waiting for its own connection: %v", ctx.Err())
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != itemID || items[0].Note == nil || items[0].Note.Content != "Export note" {
		t.Fatalf("missing favorite or metadata: %#v", items)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("connection not returned: %v", err)
	}
}
