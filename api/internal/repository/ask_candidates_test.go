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
	defer pool.Close()

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
