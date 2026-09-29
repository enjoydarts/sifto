package repository

import (
	"context"
	"testing"
)

func TestAskCandidatesByEmbeddingEmptyUserDoesNotError(t *testing.T) {
	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	repo := NewItemRepo(pool)
	got, err := repo.AskCandidatesByEmbedding(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		"AI 投資",
		[]float64{0.1, 0.2, 0.3},
		30,
		false,
		nil,
		10,
	)
	if err != nil {
		t.Fatalf("AskCandidatesByEmbedding() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 for empty user", len(got))
	}
}

func TestAskCandidatesByEmbeddingSearchesPastNewerItemsWithoutEmbeddings(t *testing.T) {
	ctx := context.Background()
	pool, err := NewPool(ctx)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	var userID, sourceID, olderID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name)
		VALUES ('ask-candidates-' || gen_random_uuid()::text || '@example.com', 'Ask Candidate Test')
		RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("delete test user: %v", err)
		}
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO sources (user_id, url, type, title)
		VALUES ($1, 'https://example.com/ask-feed', 'manual', 'Ask Candidate Test')
		RETURNING id
	`, userID).Scan(&sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		WITH newer AS (
			INSERT INTO items (source_id, url, title, status, published_at)
			SELECT $1::uuid, 'https://example.com/ask/' || $1::text || '/' || n,
			       'Newer unembedded item', 'summarized', NOW() - INTERVAL '1 day'
			FROM generate_series(1, 600) AS n
			RETURNING id
		)
		INSERT INTO item_summaries (item_id, summary, score)
		SELECT id, 'Newer summary', 0.5 FROM newer
	`, sourceID); err != nil {
		t.Fatalf("insert newer items: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO items (source_id, url, title, status, published_at)
		VALUES ($1::uuid, 'https://example.com/ask/' || $1::text || '/older', 'AI 投資', 'summarized', NOW() - INTERVAL '2 days')
		RETURNING id
	`, sourceID).Scan(&olderID); err != nil {
		t.Fatalf("insert older item: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO item_summaries (item_id, summary, score) VALUES ($1, 'AI 投資 summary', 0.5)`, olderID); err != nil {
		t.Fatalf("insert older summary: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO item_embeddings (item_id, model, dimensions, embedding)
		VALUES ($1, 'test-embedding', 3, ARRAY[0.1, 0.2, 0.3]::double precision[])
	`, olderID); err != nil {
		t.Fatalf("insert older embedding: %v", err)
	}

	repo := NewItemRepo(pool)
	got, err := repo.AskCandidatesByEmbedding(ctx, userID, "AI 投資", []float64{0.1, 0.2, 0.3}, 90, false, nil, 1)
	if err != nil {
		t.Fatalf("AskCandidatesByEmbedding() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != olderID {
		t.Fatalf("candidates = %v, want one item with ID %s", got, olderID)
	}
}
