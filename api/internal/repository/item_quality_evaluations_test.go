package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testItemQualityEvaluationRepoDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestItemQualityEvaluationRepoUpsertAndLoadLatest(t *testing.T) {
	ctx := context.Background()
	pool := testItemQualityEvaluationRepoDB(t)
	const userID = "00000000-0000-4000-8000-000000000160"
	const sourceID = "00000000-0000-4000-8000-000000000161"
	const itemID = "00000000-0000-4000-8000-000000000162"
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("clean user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, 'jev-quality-repo@example.com')`, userID); err != nil {
		t.Fatalf("prepare user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sources (id, user_id, url, type) VALUES ($1, $2, 'https://example.com/feed', 'rss')`, sourceID, userID); err != nil {
		t.Fatalf("prepare source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO items (id, source_id, url) VALUES ($1, $2, 'https://example.com/item')`, itemID, sourceID); err != nil {
		t.Fatalf("prepare item: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	repo := NewItemQualityEvaluationRepo(pool)
	base := ItemQualityEvaluationInput{
		ItemID: itemID, Kind: "facts", Provider: "jev", RequestedModel: "jev-latest", Model: "jev-20260915",
		Dimensions:     map[string]map[string]float64{"source_support": {"score": 0.95, "raw_score": 3.8, "confidence": 0.96}},
		AggregateScore: 0.95, MinimumScore: 0.95, MinimumConfidence: 0.96,
		QualityThreshold: 0.9, ConfidenceThreshold: 0.9, GatePolicyVersion: "v1", Decision: "accepted",
		InputTokens: 10, OutputTokens: 1, EstimatedCostUSD: 0.00000042, LatencyMS: 80,
	}
	if err := repo.Upsert(ctx, base); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	reason := "low_score"
	base.Decision = "escalated"
	base.EscalationReason = &reason
	base.AggregateScore = 0.89
	if err := repo.Upsert(ctx, base); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}

	got, err := repo.LoadLatestByKind(ctx, itemID, "facts")
	if err != nil {
		t.Fatalf("LoadLatestByKind() error = %v", err)
	}
	if got == nil || got.Decision != "escalated" || got.EscalationReason == nil || *got.EscalationReason != reason {
		t.Fatalf("evaluation = %#v", got)
	}
	if got.Model != "jev-20260915" || got.GatePolicyVersion != "v1" || len(got.Dimensions) != 1 {
		t.Fatalf("persisted metadata = %#v", got)
	}
}
