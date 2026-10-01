package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/inngest/inngestgo"
)

// Carry the exact candidate that Jev checks; later article retries can change it.
type D1ShadowEventData struct {
	ItemID    string   `json:"item_id"`
	SourceID  string   `json:"source_id"`
	UserID    string   `json:"user_id"`
	TriggerID string   `json:"trigger_id"`
	Reason    string   `json:"reason"`
	Kind      string   `json:"kind"`
	Attempt   int      `json:"attempt"`
	Title     string   `json:"title"`
	Content   string   `json:"content,omitempty"`
	Facts     []string `json:"facts"`
	Summary   string   `json:"summary,omitempty"`
}

func NewD1ShadowEvent(data D1ShadowEventData) inngestgo.GenericEvent[D1ShadowEventData] {
	data.Facts = append([]string(nil), data.Facts...)
	payload, _ := json.Marshal(data)
	id := fmt.Sprintf("d1-shadow-%x", sha256.Sum256(payload))
	return inngestgo.GenericEvent[D1ShadowEventData]{Name: "item/d1-shadow.requested", ID: &id, Data: data}
}

func (p *EventPublisher) SendD1ShadowE(ctx context.Context, data D1ShadowEventData) error {
	_, err := p.client.Send(ctx, NewD1ShadowEvent(data))
	return err
}
