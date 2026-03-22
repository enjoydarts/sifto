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
	RunID  string `json:"run_id"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type itemSearchBackfillRunEvent struct {
	RunID string `json:"run_id"`
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
	runRepo := repository.NewSearchBackfillRunRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "item-search-backfill", Name: "Backfill Item Search Documents"},
		inngestgo.EventTrigger("item/search.backfill", nil),
		func(ctx context.Context, input inngestgo.Input[itemSearchBackfillEvent]) (any, error) {
			if input.Event.Data.RunID == "" {
				return nil, fmt.Errorf("run_id is required")
			}
			docs, err := docRepo.ListSummarizedPage(ctx, input.Event.Data.Offset, input.Event.Data.Limit)
			if err != nil {
				if _, markErr := runRepo.MarkBatchFailed(ctx, input.Event.Data.RunID, err.Error()); markErr != nil {
					return nil, markErr
				}
				return map[string]any{
					"run_id": input.Event.Data.RunID,
					"offset": input.Event.Data.Offset,
					"limit":  input.Event.Data.Limit,
					"status": "failed",
					"error":  err.Error(),
				}, nil
			}
			if err := search.UpsertItemDocuments(ctx, docs); err != nil {
				if _, markErr := runRepo.MarkBatchFailed(ctx, input.Event.Data.RunID, err.Error()); markErr != nil {
					return nil, markErr
				}
				return map[string]any{
					"run_id": input.Event.Data.RunID,
					"offset": input.Event.Data.Offset,
					"limit":  input.Event.Data.Limit,
					"status": "failed",
					"error":  err.Error(),
				}, nil
			}
			if _, err := runRepo.MarkBatchSucceeded(ctx, input.Event.Data.RunID, len(docs)); err != nil {
				return nil, err
			}
			return map[string]any{
				"run_id": input.Event.Data.RunID,
				"offset": input.Event.Data.Offset,
				"limit":  input.Event.Data.Limit,
				"count":  len(docs),
				"status": "upserted",
			}, nil
		},
	)
}

func itemSearchBackfillRunFn(client inngestgo.Client, db *pgxpool.Pool) (inngestgo.ServableFunction, error) {
	runRepo := repository.NewSearchBackfillRunRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "item-search-backfill-run", Name: "Queue Item Search Backfill Run"},
		inngestgo.EventTrigger("item/search.backfill.run", nil),
		func(ctx context.Context, input inngestgo.Input[itemSearchBackfillRunEvent]) (any, error) {
			if input.Event.Data.RunID == "" {
				return nil, fmt.Errorf("run_id is required")
			}

			run, err := runRepo.GetByID(ctx, input.Event.Data.RunID)
			if err != nil {
				return nil, err
			}
			if run.QueuedBatches == 0 {
				return map[string]any{
					"run_id":         run.ID,
					"queued_batches": 0,
					"status":         "completed",
				}, nil
			}

			for batch := 0; batch < run.QueuedBatches; batch++ {
				offset := run.RequestedOffset + batch*run.BatchSize
				if _, err := client.Send(ctx, inngestgo.Event{
					Name: "item/search.backfill",
					Data: map[string]any{
						"run_id": run.ID,
						"offset": offset,
						"limit":  run.BatchSize,
					},
				}); err != nil {
					if _, markErr := runRepo.MarkFanoutFailed(ctx, run.ID, err.Error()); markErr != nil {
						return nil, markErr
					}
					return map[string]any{
						"run_id":         run.ID,
						"queued_batches": batch,
						"status":         "failed",
						"error":          err.Error(),
					}, nil
				}
			}

			if _, err := runRepo.MarkRunning(ctx, run.ID); err != nil {
				return nil, err
			}

			return map[string]any{
				"run_id":         run.ID,
				"queued_batches": run.QueuedBatches,
				"status":         "queued",
			}, nil
		},
	)
}
