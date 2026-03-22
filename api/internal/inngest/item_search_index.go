package inngest

import (
	"context"
	"fmt"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/service"
	"github.com/inngest/inngestgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type itemSearchUpsertEvent struct {
	ItemID string `json:"item_id"`
}

type itemSearchBackfillEvent struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

func itemSearchUpsertFn(client inngestgo.Client, db *pgxpool.Pool, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	docRepo := repository.NewItemSearchDocumentRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "item-search-upsert", Name: "Upsert Item Search Document"},
		inngestgo.EventTrigger("item/search.upsert", nil),
		func(ctx context.Context, input inngestgo.Input[itemSearchUpsertEvent]) (any, error) {
			if input.Event.Data.ItemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}

			doc, err := docRepo.GetByItemID(ctx, input.Event.Data.ItemID)
			if err != nil {
				if err == pgx.ErrNoRows {
					return map[string]any{"item_id": input.Event.Data.ItemID, "status": "missing"}, nil
				}
				return nil, err
			}
			if err := search.UpsertItemDocuments(ctx, []model.ItemSearchDocument{*doc}); err != nil {
				return nil, err
			}
			return map[string]any{"item_id": input.Event.Data.ItemID, "status": "upserted"}, nil
		},
	)
}

func itemSearchDeleteFn(client inngestgo.Client, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "item-search-delete", Name: "Delete Item Search Document"},
		inngestgo.EventTrigger("item/search.delete", nil),
		func(ctx context.Context, input inngestgo.Input[itemSearchUpsertEvent]) (any, error) {
			if input.Event.Data.ItemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}
			if err := search.DeleteItemDocuments(ctx, []string{input.Event.Data.ItemID}); err != nil {
				return nil, err
			}
			return map[string]any{"item_id": input.Event.Data.ItemID, "status": "deleted"}, nil
		},
	)
}

func itemSearchBackfillFn(client inngestgo.Client, db *pgxpool.Pool, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	docRepo := repository.NewItemSearchDocumentRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "item-search-backfill", Name: "Backfill Item Search Documents"},
		inngestgo.EventTrigger("item/search.backfill", nil),
		func(ctx context.Context, input inngestgo.Input[itemSearchBackfillEvent]) (any, error) {
			docs, err := docRepo.ListSummarizedPage(ctx, input.Event.Data.Offset, input.Event.Data.Limit)
			if err != nil {
				return nil, err
			}
			if err := search.UpsertItemDocuments(ctx, docs); err != nil {
				return nil, err
			}
			return map[string]any{
				"offset": input.Event.Data.Offset,
				"limit":  input.Event.Data.Limit,
				"count":  len(docs),
				"status": "upserted",
			}, nil
		},
	)
}
