package repository

import (
	"context"
	"testing"
)

func TestLLMUsageLogRepoAcceptsJevPrecheckPurposes(t *testing.T) {
	ctx := context.Background()
	pool := testItemQualityEvaluationRepoDB(t)
	const userID = "00000000-0000-4000-8000-000000000163"
	const model = "jev-purpose-constraint-test"
	if _, err := pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = 'jev' AND model = $1`, model); err != nil {
		t.Fatalf("clean usage logs: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("clean user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, 'jev-usage-repo@example.com')`, userID); err != nil {
		t.Fatalf("prepare user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM llm_usage_logs WHERE provider = 'jev' AND model = $1`, model)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	repo := NewLLMUsageLogRepo(pool)
	usageUserID := userID
	for _, purpose := range []string{"facts_check_precheck", "faithfulness_check_precheck"} {
		purpose := purpose
		t.Run(purpose, func(t *testing.T) {
			key := "jev-purpose-constraint-test-" + purpose
			err := repo.Insert(ctx, LLMUsageLogInput{
				IdempotencyKey:   &key,
				UserID:           &usageUserID,
				Provider:         "jev",
				Model:            model,
				PricingSource:    "typesafe_jev_test",
				Purpose:          purpose,
				InputTokens:      100,
				EstimatedCostUSD: 0.0000042,
			})
			if err != nil {
				t.Fatalf("Insert(%q) error = %v", purpose, err)
			}
		})
	}

	providers, err := repo.ProviderSummaryCurrentMonthByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ProviderSummaryCurrentMonthByUser() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Provider != "jev" || providers[0].Calls != 2 || providers[0].EstimatedCostUSD <= 0 {
		t.Fatalf("provider summary = %#v", providers)
	}
	purposes, err := repo.PurposeSummaryCurrentMonthByUser(ctx, userID)
	if err != nil {
		t.Fatalf("PurposeSummaryCurrentMonthByUser() error = %v", err)
	}
	if len(purposes) != 2 {
		t.Fatalf("purpose summary = %#v", purposes)
	}
}
