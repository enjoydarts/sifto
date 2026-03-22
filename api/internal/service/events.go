package service

import (
	"context"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/inngest/inngestgo"
)

type EventPublisher struct {
	client inngestgo.Client
}

func NewEventPublisher() (*EventPublisher, error) {
	client, err := NewInngestClient("sifto-api")
	if err != nil {
		return nil, err
	}
	return &EventPublisher{client: client}, nil
}

func (p *EventPublisher) SendItemCreated(ctx context.Context, itemID, sourceID, url string) {
	_ = p.SendItemCreatedWithReasonE(ctx, itemID, sourceID, url, nil, "unknown")
}

func (p *EventPublisher) SendItemCreatedE(ctx context.Context, itemID, sourceID, url string) error {
	return p.SendItemCreatedWithReasonE(ctx, itemID, sourceID, url, nil, "unknown")
}

func NewItemCreatedEvent(itemID, sourceID, url string, title *string, reason string) inngestgo.Event {
	data := map[string]any{
		"item_id":    itemID,
		"source_id":  sourceID,
		"url":        url,
		"trigger_id": uuid.NewString(),
		"reason":     reason,
	}
	if title != nil {
		data["title"] = *title
	}
	return inngestgo.Event{
		Name: "item/created",
		Data: data,
	}
}

func (p *EventPublisher) SendItemCreatedWithReasonE(ctx context.Context, itemID, sourceID, url string, title *string, reason string) error {
	if p == nil {
		return nil
	}
	if _, err := p.client.Send(ctx, NewItemCreatedEvent(itemID, sourceID, url, title, reason)); err != nil {
		log.Printf("send item/created: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendDigestCreatedE(ctx context.Context, digestID, userID, to string) error {
	if p == nil {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "digest/created",
		Data: map[string]any{
			"digest_id": digestID,
			"user_id":   userID,
			"to":        to,
		},
	}); err != nil {
		log.Printf("send digest/created: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendItemEmbedE(ctx context.Context, itemID, sourceID string) error {
	if p == nil {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "item/embed",
		Data: map[string]any{
			"item_id":   itemID,
			"source_id": sourceID,
		},
	}); err != nil {
		log.Printf("send item/embed: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendItemSearchUpsertE(ctx context.Context, itemID string) error {
	if p == nil || strings.TrimSpace(itemID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "item/search.upsert",
		Data: map[string]any{
			"item_id": itemID,
		},
	}); err != nil {
		log.Printf("send item/search.upsert: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendItemSearchDeleteE(ctx context.Context, itemID string) error {
	if p == nil || strings.TrimSpace(itemID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "item/search.delete",
		Data: map[string]any{
			"item_id": itemID,
		},
	}); err != nil {
		log.Printf("send item/search.delete: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendItemSearchBackfillE(ctx context.Context, offset, limit int) error {
	if p == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "item/search.backfill",
		Data: map[string]any{
			"offset": offset,
			"limit":  limit,
		},
	}); err != nil {
		log.Printf("send item/search.backfill: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendItemSearchBackfillRunE(ctx context.Context, runID string) error {
	if p == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "item/search.backfill.run",
		Data: map[string]any{
			"run_id": runID,
		},
	}); err != nil {
		log.Printf("send item/search.backfill.run: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendSearchSuggestionArticleUpsertE(ctx context.Context, itemID string) error {
	if p == nil || strings.TrimSpace(itemID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "search/suggestions.article.upsert",
		Data: map[string]any{
			"item_id": itemID,
		},
	}); err != nil {
		log.Printf("send search/suggestions.article.upsert: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendSearchSuggestionArticleDeleteE(ctx context.Context, itemID string) error {
	if p == nil || strings.TrimSpace(itemID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "search/suggestions.article.delete",
		Data: map[string]any{
			"item_id": itemID,
		},
	}); err != nil {
		log.Printf("send search/suggestions.article.delete: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendSearchSuggestionSourceUpsertE(ctx context.Context, sourceID string) error {
	if p == nil || strings.TrimSpace(sourceID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "search/suggestions.source.upsert",
		Data: map[string]any{
			"source_id": sourceID,
		},
	}); err != nil {
		log.Printf("send search/suggestions.source.upsert: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendSearchSuggestionSourceDeleteE(ctx context.Context, sourceID string) error {
	if p == nil || strings.TrimSpace(sourceID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "search/suggestions.source.delete",
		Data: map[string]any{
			"source_id": sourceID,
		},
	}); err != nil {
		log.Printf("send search/suggestions.source.delete: %v", err)
		return err
	}
	return nil
}

func (p *EventPublisher) SendSearchSuggestionTopicsRefreshE(ctx context.Context, userID string) error {
	if p == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "search/suggestions.topics.refresh",
		Data: map[string]any{
			"user_id": userID,
		},
	}); err != nil {
		log.Printf("send search/suggestions.topics.refresh: %v", err)
		return err
	}
	return nil
}
