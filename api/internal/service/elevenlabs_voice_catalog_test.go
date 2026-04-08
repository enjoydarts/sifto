package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestElevenLabsVoiceCatalogServiceFetchVoices(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/shared-voices" {
			t.Fatalf("path = %s, want /v1/shared-voices", r.URL.Path)
		}
		if got, want := r.Header.Get("xi-api-key"), "test-key"; got != want {
			t.Fatalf("xi-api-key = %q, want %q", got, want)
		}
		requestCount++
		if got := r.URL.Query().Get("page_size"); got != "100" {
			t.Fatalf("page_size = %q, want 100", got)
		}
		if got := r.URL.Query().Get("language"); got != "ja" {
			t.Fatalf("language = %q, want ja", got)
		}
		if got := r.URL.Query().Get("locale"); got != "ja-JP" {
			t.Fatalf("locale = %q, want ja-JP", got)
		}
		if got := r.URL.Query().Get("sort"); got != "trending" {
			t.Fatalf("sort = %q, want trending", got)
		}
		switch requestCount {
		case 1:
			if got := r.URL.Query().Get("page"); got != "0" {
				t.Fatalf("page = %q, want 0", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"has_more": true,
				"voices": []map[string]any{
					{
						"voice_id":    "voice-b",
						"name":        "Beta",
						"description": "Second",
						"category":    "premade",
						"preview_url": "https://example.com/b-default.mp3",
						"language":    "ja",
						"locale":      "ja-JP",
						"verified_languages": []map[string]any{
							{"language": "ja", "locale": "ja-JP", "preview_url": "https://example.com/b-ja.mp3"},
							{"language": "en", "locale": "en-US", "preview_url": "https://example.com/b-en.mp3"},
						},
					},
					{
						"voice_id":    "voice-en",
						"name":        "English",
						"description": "Not Japanese",
						"category":    "premade",
						"preview_url": "https://example.com/en.mp3",
						"language":    "en",
						"locale":      "en-US",
					},
				},
			})
		case 2:
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Fatalf("page = %q, want 1", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"has_more": false,
				"voices": []map[string]any{
					{
						"voice_id":    "voice-a",
						"name":        "Alpha",
						"description": "First",
						"category":    "premade",
						"preview_url": "https://example.com/a-default.mp3",
						"language":    "ja",
						"locale":      "ja-JP",
						"verified_languages": []map[string]any{
							{"language": "ja", "locale": "ja-JP", "preview_url": "https://example.com/a-ja.mp3"},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected request count %d", requestCount)
		}
	}))
	defer srv.Close()

	svc := &ElevenLabsVoiceCatalogService{
		baseURL: srv.URL,
		http:    srv.Client(),
	}

	rows, err := svc.FetchVoices(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("FetchVoices() error = %v", err)
	}
	if len(rows.Voices) != 2 {
		t.Fatalf("len(rows.Voices) = %d, want 2", len(rows.Voices))
	}
	if rows.Voices[0].VoiceID != "voice-b" || rows.Voices[1].VoiceID != "voice-a" {
		t.Fatalf("voices = %#v, want trending order preserved", rows.Voices)
	}
	if rows.Source != "elevenlabs_shared_voices_ja" {
		t.Fatalf("source = %q, want elevenlabs_shared_voices_ja", rows.Source)
	}
	if requestCount != 2 {
		t.Fatalf("requestCount = %d, want 2", requestCount)
	}
	if strings.Join(rows.Voices[0].Languages, ",") != "ja-JP,ja" {
		t.Fatalf("languages = %#v, want ja-JP,ja", rows.Voices[0].Languages)
	}
	if rows.Voices[0].PreviewURL != "https://example.com/b-ja.mp3" {
		t.Fatalf("preview = %q, want japanese preview", rows.Voices[0].PreviewURL)
	}
}
