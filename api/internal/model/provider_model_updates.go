package model

import "time"

type ProviderModelSnapshot struct {
	Provider  string    `json:"provider"`
	Models    []string  `json:"models"`
	FetchedAt time.Time `json:"fetched_at"`
	Status    string    `json:"status"`
	Error     *string   `json:"error,omitempty"`
}

type ProviderModelChangeEvent struct {
	ID         string         `json:"id"`
	Provider   string         `json:"provider"`
	ChangeType string         `json:"change_type"`
	ModelID    string         `json:"model_id"`
	DetectedAt time.Time      `json:"detected_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
