package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestXAIVoiceCatalogServiceFetchVoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tts/voices" {
			t.Fatalf("path = %s, want /v1/tts/voices", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer test-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"voices": []map[string]any{
				{
					"voice_id":    "voice-1",
					"name":        "Calm",
					"description": "Warm",
					"language":    "en",
				},
			},
		})
	}))
	defer srv.Close()

	svc := NewXAIVoiceCatalogServiceWithBaseURL(srv.URL)
	rows, err := svc.FetchVoices(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("FetchVoices() error = %v", err)
	}
	if len(rows) != 1 || rows[0].VoiceID != "voice-1" {
		t.Fatalf("rows = %#v, want voice-1", rows)
	}
}
