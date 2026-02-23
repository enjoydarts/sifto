package service

import (
	"context"
	"log"

	"github.com/inngest/inngestgo"
)

type EventPublisher struct {
	client inngestgo.Client
}

func NewEventPublisher() (*EventPublisher, error) {
	client, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: "sifto-api",
	})
	if err != nil {
		return nil, err
	}
	return &EventPublisher{client: client}, nil
}

func (p *EventPublisher) SendItemCreated(ctx context.Context, itemID, sourceID, url string) {
	if p == nil {
		return
	}
	if _, err := p.client.Send(ctx, inngestgo.Event{
		Name: "item/created",
		Data: map[string]any{
			"item_id":   itemID,
			"source_id": sourceID,
			"url":       url,
		},
	}); err != nil {
		log.Printf("send item/created: %v", err)
	}
}

