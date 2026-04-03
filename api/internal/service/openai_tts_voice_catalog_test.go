package service

import (
	"context"
	"encoding/json"
	"testing"
)

func TestOpenAITTSVoiceCatalogServiceFetchVoices(t *testing.T) {
	svc := NewOpenAITTSVoiceCatalogService()

	rows, err := svc.FetchVoices(context.Background())
	if err != nil {
		t.Fatalf("FetchVoices() error = %v", err)
	}
	if len(rows) != 13 {
		t.Fatalf("len(rows) = %d, want 13", len(rows))
	}
	if rows[0].VoiceID != "alloy" || rows[len(rows)-1].VoiceID != "cedar" {
		t.Fatalf("rows = %#v, want sorted builtin voices", rows)
	}
	if rows[0].Language != "multilingual" {
		t.Fatalf("rows[0].Language = %q, want multilingual", rows[0].Language)
	}
}

func TestOpenAITTSVoiceCatalogServiceFetchVoicesIncludesSupportedModelsMetadata(t *testing.T) {
	svc := NewOpenAITTSVoiceCatalogService()

	rows, err := svc.FetchVoices(context.Background())
	if err != nil {
		t.Fatalf("FetchVoices() error = %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(rows[0].MetadataJSON, &metadata); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	models, ok := metadata["supported_models"].([]any)
	if !ok || len(models) == 0 {
		t.Fatalf("metadata = %#v, want supported_models", metadata)
	}
}
