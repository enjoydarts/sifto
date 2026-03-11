package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func loadFactsCheck(ctx context.Context, db *pgxpool.Pool, itemID string) (*model.FactsCheck, error) {
	var check model.FactsCheck
	err := db.QueryRow(ctx, `
		SELECT id, item_id, final_result, retry_count, short_comment, created_at, updated_at
		FROM item_facts_checks
		WHERE item_id = $1`, itemID,
	).Scan(&check.ID, &check.ItemID, &check.FinalResult, &check.RetryCount, &check.ShortComment, &check.CreatedAt, &check.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &check, nil
}

func loadFaithfulnessCheck(ctx context.Context, db *pgxpool.Pool, itemID string) (*model.SummaryFaithfulnessCheck, error) {
	var check model.SummaryFaithfulnessCheck
	err := db.QueryRow(ctx, `
		SELECT id, item_id, final_result, retry_count, short_comment, created_at, updated_at
		FROM summary_faithfulness_checks
		WHERE item_id = $1`, itemID,
	).Scan(&check.ID, &check.ItemID, &check.FinalResult, &check.RetryCount, &check.ShortComment, &check.CreatedAt, &check.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &check, nil
}
