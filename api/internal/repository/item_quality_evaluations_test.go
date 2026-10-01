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
		Dimensions:       map[string]map[string]float64{"source_support": {"score": 0.95, "raw_score": 3.8, "confidence": 0.96}},
		Signals:          map[string]map[string]float64{"has_unsupported_fact": {"probability": 0.02}},
		SignalThresholds: map[string]float64{"has_unsupported_fact": 0.10},
		AggregateScore:   0.95, MinimumScore: 0.95, MinimumConfidence: 0.96,
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
	if got.Signals["has_unsupported_fact"].Probability != 0.02 || got.SignalThresholds["has_unsupported_fact"] != 0.10 {
		t.Fatalf("persisted signals = %#v thresholds = %#v", got.Signals, got.SignalThresholds)
	}
	d1 := base
	d1.Provider = "d1"
	d1.RequestedModel = "d1:free"
	d1.Model = "d1:free"
	d1.GatePolicyVersion = "d1-shadow-v1"
	d1.Decision = "accepted"
	d1.EscalationReason = nil
	if err := repo.Upsert(ctx, d1); err != nil {
		t.Fatalf("D1 Upsert() error = %v", err)
	}
	jevAgain, err := repo.LoadLatestByKind(ctx, itemID, "facts")
	if err != nil || jevAgain == nil || jevAgain.Provider != "jev" || jevAgain.Decision != "escalated" {
		t.Fatalf("Jev after D1 = %#v, %v", jevAgain, err)
	}
	d1Got, err := repo.LoadLatestByKindAndProvider(ctx, itemID, "facts", "d1")
	if err != nil || d1Got == nil || d1Got.Model != "d1:free" || d1Got.Decision != "accepted" {
		t.Fatalf("D1 evaluation = %#v, %v", d1Got, err)
	}
	// An asynchronous result from an older processing run must not replace a
	// result for the newer candidate, even when both use attempt zero.
	currentTrigger := "new-processing-run"
	executionUserID, executionItemID := userID, itemID
	executionRepo := NewLLMExecutionEventRepo(pool)
	if err := executionRepo.Insert(ctx, LLMExecutionEventInput{UserID: &executionUserID, ItemID: &executionItemID, TriggerID: &currentTrigger,
		Provider: "openai", Model: "test", Purpose: "facts", Status: "success"}); err != nil {
		t.Fatalf("prepare execution: %v", err)
	}
	oldTrigger := "old-processing-run"
	d1.CurrentTriggerID = &oldTrigger
	d1.Decision = "error"
	if err := repo.Upsert(ctx, d1); err != nil {
		t.Fatalf("stale D1 Upsert() error = %v", err)
	}
	d1Got, err = repo.LoadLatestByKindAndProvider(ctx, itemID, "facts", "d1")
	if err != nil || d1Got == nil || d1Got.Decision != "accepted" {
		t.Fatalf("stale D1 overwrote current evaluation: %#v, %v", d1Got, err)
	}
	d1.CurrentTriggerID = &currentTrigger
	if err := repo.Upsert(ctx, d1); err != nil {
		t.Fatalf("current D1 Upsert() error = %v", err)
	}
	d1Got, err = repo.LoadLatestByKindAndProvider(ctx, itemID, "facts", "d1")
	if err != nil || d1Got == nil || d1Got.Decision != "error" {
		t.Fatalf("current D1 result was skipped: %#v, %v", d1Got, err)
	}
}
