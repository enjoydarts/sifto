package model

import "time"

func ExcludeFromProviderModelSnapshots(provider string) bool {
	switch provider {
	case "aivis", "openrouter", "poe", "featherless", "deepinfra":
		return true
	default:
		return false
	}
}

type ProviderModelSnapshot struct {
	Provider  string    `json:"provider"`
	Models    []string  `json:"models"`
	FetchedAt time.Time `json:"fetched_at"`
	Status    string    `json:"status"`
	Error     *string   `json:"error,omitempty"`
}

type ProviderModelSnapshotEntry struct {
	Provider  string    `json:"provider"`
	ModelID   string    `json:"model_id"`
	FetchedAt time.Time `json:"fetched_at"`
	Status    string    `json:"status"`
	Error     *string   `json:"error,omitempty"`
}

type ProviderModelSnapshotList struct {
	Items     []ProviderModelSnapshotEntry `json:"items"`
	Providers []string                     `json:"providers"`
	Total     int                          `json:"total"`
	Limit     int                          `json:"limit"`
	Offset    int                          `json:"offset"`
}

type ProviderModelChangeEvent struct {
	ID         string         `json:"id"`
	Provider   string         `json:"provider"`
	ChangeType string         `json:"change_type"`
	ModelID    string         `json:"model_id"`
	DetectedAt time.Time      `json:"detected_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ProviderModelChangeSummary struct {
	Provider            string                     `json:"provider"`
	DetectedAt          time.Time                  `json:"detected_at"`
	Trigger             string                     `json:"trigger"`
	Added               []ProviderModelChangeEvent `json:"added"`
	Constrained         []ProviderModelChangeEvent `json:"constrained"`
	AvailabilityChanged []ProviderModelChangeEvent `json:"availability_changed"`
	GatedChanged        []ProviderModelChangeEvent `json:"gated_changed"`
	PricingChanged      []ProviderModelChangeEvent `json:"pricing_changed"`
	ContextChanged      []ProviderModelChangeEvent `json:"context_changed"`
	Removed             []ProviderModelChangeEvent `json:"removed"`
}
