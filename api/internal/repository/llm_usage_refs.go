package repository

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func loadLatestItemLLMUsage(ctx context.Context, db *pgxpool.Pool, itemID, purpose string) (*model.ItemSummaryLLM, error) {
	var llm model.ItemSummaryLLM
	err := db.QueryRow(ctx, `
		SELECT provider, model, pricing_source, created_at
		FROM llm_usage_logs
		WHERE item_id = $1
		  AND purpose = $2
		ORDER BY created_at DESC
		LIMIT 1`, itemID, purpose,
	).Scan(&llm.Provider, &llm.Model, &llm.PricingSource, &llm.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &llm, nil
}

func loadLatestDigestLLMUsage(ctx context.Context, db *pgxpool.Pool, digestID, purpose string) (*model.ItemSummaryLLM, error) {
	var llm model.ItemSummaryLLM
	err := db.QueryRow(ctx, `
		SELECT provider, model, pricing_source, created_at
		FROM llm_usage_logs
		WHERE digest_id = $1
		  AND purpose = $2
		ORDER BY created_at DESC
		LIMIT 1`, digestID, purpose,
	).Scan(&llm.Provider, &llm.Model, &llm.PricingSource, &llm.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &llm, nil
}

func loadLatestItemEmbeddingModel(ctx context.Context, db *pgxpool.Pool, itemID string) (*string, error) {
	var modelID string
	err := db.QueryRow(ctx, `
		SELECT model
		FROM item_embeddings
		WHERE item_id = $1
		ORDER BY updated_at DESC
		LIMIT 1`, itemID,
	).Scan(&modelID)
	if err != nil {
		return nil, err
	}
	return &modelID, nil
}
