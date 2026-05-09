package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCartesiaTTSCatalogServiceFetchCatalogFiltersJapaneseVoices(t *testing.T) {
	var requestedPath string
	var requestedLimit string
	var requestedLanguage string
	var requestedAuth string
	var requestedVersion string
	var requestedExpand string
	var detailRequested bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/voices/") {
			detailRequested = true
			requestedExpand = r.URL.Query().Get("expand[]")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               "ja-voice-1",
				"name":             "Japanese Voice",
				"description":      "Japanese narration",
				"language":         "ja",
				"preview_file_url": "https://example.com/ja-preview.mp3",
			})
			return
		}

		requestedPath = r.URL.Path
		requestedLimit = r.URL.Query().Get("limit")
		requestedLanguage = r.URL.Query().Get("language")
		requestedAuth = r.Header.Get("Authorization")
		requestedVersion = r.Header.Get("Cartesia-Version")

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":          "ja-voice-1",
					"name":        "Japanese Voice",
					"description": "Japanese narration",
					"language":    "",
				},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	svc := &CartesiaTTSCatalogService{
		baseURL: server.URL,
		http:    server.Client(),
	}

	resp, err := svc.FetchCatalog(context.Background(), "cartesia-key")
	if err != nil {
		t.Fatalf("FetchCatalog() error = %v", err)
	}

	if requestedPath != "/voices" {
		t.Fatalf("path = %q, want /voices", requestedPath)
	}
	if requestedLimit != "100" {
		t.Fatalf("limit = %q, want 100", requestedLimit)
	}
	if requestedLanguage != "ja" {
		t.Fatalf("language = %q, want ja", requestedLanguage)
	}
	if requestedAuth != "Bearer cartesia-key" {
		t.Fatalf("Authorization = %q, want Bearer cartesia-key", requestedAuth)
	}
	if requestedVersion != "2026-03-01" {
		t.Fatalf("Cartesia-Version = %q, want 2026-03-01", requestedVersion)
	}
	if !detailRequested {
		t.Fatal("voice detail endpoint was not requested for preview_file_url")
	}
	if requestedExpand != "preview_file_url" {
		t.Fatalf("expand[] = %q, want preview_file_url", requestedExpand)
	}
	if resp.Provider != "cartesia" {
		t.Fatalf("Provider = %q, want cartesia", resp.Provider)
	}
	modelIDs := make([]string, 0, len(resp.Models))
	for _, model := range resp.Models {
		modelIDs = append(modelIDs, model.ModelID)
	}
	if !containsString(modelIDs, "sonic-3.5") || !containsString(modelIDs, "sonic-3") || !containsString(modelIDs, "sonic-turbo") {
		t.Fatalf("Models = %#v, want stable sonic-3.5, sonic-3, and sonic-turbo", resp.Models)
	}
	if len(resp.Voices) != 1 {
		t.Fatalf("Voices len = %d, want 1: %#v", len(resp.Voices), resp.Voices)
	}
	if resp.Voices[0].VoiceID != "ja-voice-1" {
		t.Fatalf("Voice = %#v, want Japanese voice", resp.Voices[0])
	}
	if resp.Voices[0].PreviewURL != "https://example.com/ja-preview.mp3" {
		t.Fatalf("PreviewURL = %q, want preview url", resp.Voices[0].PreviewURL)
	}
}

func TestCartesiaTTSCatalogServiceFetchVoicePreview(t *testing.T) {
	var detailRequested bool
	var previewRequested bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/voices/ja-voice-1":
			detailRequested = true
			if got := r.URL.Query().Get("expand[]"); got != "preview_file_url" {
				t.Fatalf("expand[] = %q, want preview_file_url", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               "ja-voice-1",
				"name":             "Japanese Voice",
				"description":      "Japanese narration",
				"language":         "ja",
				"preview_file_url": serverURL(r) + "/preview/ja-voice-1.mp3",
			})
		case r.URL.Path == "/preview/ja-voice-1.mp3":
			previewRequested = true
			if got := r.Header.Get("Authorization"); got != "Bearer cartesia-key" {
				t.Fatalf("preview Authorization = %q, want Bearer cartesia-key", got)
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("mp3-bytes"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	svc := &CartesiaTTSCatalogService{
		baseURL: server.URL,
		http:    server.Client(),
	}

	audio, err := svc.FetchVoicePreview(context.Background(), "cartesia-key", "ja-voice-1")
	if err != nil {
		t.Fatalf("FetchVoicePreview() error = %v", err)
	}
	if !detailRequested || !previewRequested {
		t.Fatalf("detailRequested=%v previewRequested=%v, want both true", detailRequested, previewRequested)
	}
	if audio.ContentType != "audio/mpeg" {
		t.Fatalf("ContentType = %q, want audio/mpeg", audio.ContentType)
	}
	if string(audio.Bytes) != "mp3-bytes" {
		t.Fatalf("Bytes = %q, want mp3-bytes", string(audio.Bytes))
	}
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
