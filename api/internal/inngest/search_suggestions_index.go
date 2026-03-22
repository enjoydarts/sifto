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

type searchSuggestionArticleEvent struct {
	ItemID string `json:"item_id"`
}

type searchSuggestionSourceEvent struct {
	SourceID string `json:"source_id"`
}

type searchSuggestionTopicsRefreshEvent struct {
	UserID string `json:"user_id"`
}

func searchSuggestionArticleUpsertFn(client inngestgo.Client, db *pgxpool.Pool, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	docRepo := repository.NewSearchSuggestionDocumentRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "search-suggestion-article-upsert", Name: "Upsert Search Suggestion Article"},
		inngestgo.EventTrigger("search/suggestions.article.upsert", nil),
		func(ctx context.Context, input inngestgo.Input[searchSuggestionArticleEvent]) (any, error) {
			itemID := input.Event.Data.ItemID
			if itemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}

			doc, err := docRepo.GetArticleByItemID(ctx, itemID)
			if err != nil {
				if err == pgx.ErrNoRows {
					if err := search.DeleteSearchSuggestionDocuments(ctx, []string{repository.SearchSuggestionArticleDocumentID(itemID)}); err != nil {
						return nil, err
					}
					return map[string]any{"item_id": itemID, "status": "deleted"}, nil
				}
				return nil, err
			}
			if err := search.UpsertSearchSuggestionDocuments(ctx, []model.SearchSuggestionDocument{*doc}); err != nil {
				return nil, err
			}
			return map[string]any{"item_id": itemID, "status": "upserted"}, nil
		},
	)
}

func searchSuggestionArticleDeleteFn(client inngestgo.Client, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "search-suggestion-article-delete", Name: "Delete Search Suggestion Article"},
		inngestgo.EventTrigger("search/suggestions.article.delete", nil),
		func(ctx context.Context, input inngestgo.Input[searchSuggestionArticleEvent]) (any, error) {
			itemID := input.Event.Data.ItemID
			if itemID == "" {
				return nil, fmt.Errorf("item_id is required")
			}
			if err := search.DeleteSearchSuggestionDocuments(ctx, []string{repository.SearchSuggestionArticleDocumentID(itemID)}); err != nil {
				return nil, err
			}
			return map[string]any{"item_id": itemID, "status": "deleted"}, nil
		},
	)
}

func searchSuggestionSourceUpsertFn(client inngestgo.Client, db *pgxpool.Pool, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	docRepo := repository.NewSearchSuggestionDocumentRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "search-suggestion-source-upsert", Name: "Upsert Search Suggestion Source"},
		inngestgo.EventTrigger("search/suggestions.source.upsert", nil),
		func(ctx context.Context, input inngestgo.Input[searchSuggestionSourceEvent]) (any, error) {
			sourceID := input.Event.Data.SourceID
			if sourceID == "" {
				return nil, fmt.Errorf("source_id is required")
			}

			doc, err := docRepo.GetSourceBySourceID(ctx, sourceID)
			if err != nil {
				if err == pgx.ErrNoRows {
					if err := search.DeleteSearchSuggestionDocumentsByFilter(ctx, "source_id = "+service.QuoteMeilisearchFilter(sourceID)); err != nil {
						return nil, err
					}
					return map[string]any{"source_id": sourceID, "status": "deleted"}, nil
				}
				return nil, err
			}
			if err := search.UpsertSearchSuggestionDocuments(ctx, []model.SearchSuggestionDocument{*doc}); err != nil {
				return nil, err
			}
			return map[string]any{"source_id": sourceID, "status": "upserted"}, nil
		},
	)
}

func searchSuggestionSourceDeleteFn(client inngestgo.Client, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "search-suggestion-source-delete", Name: "Delete Search Suggestion Source"},
		inngestgo.EventTrigger("search/suggestions.source.delete", nil),
		func(ctx context.Context, input inngestgo.Input[searchSuggestionSourceEvent]) (any, error) {
			sourceID := input.Event.Data.SourceID
			if sourceID == "" {
				return nil, fmt.Errorf("source_id is required")
			}
			if err := search.DeleteSearchSuggestionDocumentsByFilter(ctx, "source_id = "+service.QuoteMeilisearchFilter(sourceID)); err != nil {
				return nil, err
			}
			return map[string]any{"source_id": sourceID, "status": "deleted"}, nil
		},
	)
}

func searchSuggestionTopicsRefreshFn(client inngestgo.Client, db *pgxpool.Pool, search *service.MeilisearchService) (inngestgo.ServableFunction, error) {
	docRepo := repository.NewSearchSuggestionDocumentRepo(db)

	return inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "search-suggestion-topics-refresh", Name: "Refresh Search Suggestion Topics"},
		inngestgo.EventTrigger("search/suggestions.topics.refresh", nil),
		func(ctx context.Context, input inngestgo.Input[searchSuggestionTopicsRefreshEvent]) (any, error) {
			userID := input.Event.Data.UserID
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}

			if err := search.DeleteSearchSuggestionDocumentsByFilter(
				ctx,
				"user_id = "+service.QuoteMeilisearchFilter(userID)+" AND kind = topic",
			); err != nil {
				return nil, err
			}

			docs, err := docRepo.ListTopicsByUser(ctx, userID)
			if err != nil {
				return nil, err
			}
			if err := search.UpsertSearchSuggestionDocuments(ctx, docs); err != nil {
				return nil, err
			}
			return map[string]any{"user_id": userID, "count": len(docs), "status": "refreshed"}, nil
		},
	)
}
