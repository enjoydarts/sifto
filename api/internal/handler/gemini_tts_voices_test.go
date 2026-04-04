package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestGeminiTTSVoicesHandlerList(t *testing.T) {
	handler := NewGeminiTTSVoicesHandler(service.NewGeminiTTSVoiceCatalogService())

	req := httptest.NewRequest(http.MethodGet, "/api/gemini-tts-voices", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		CatalogName string `json:"catalog_name"`
		Provider    string `json:"provider"`
		Source      string `json:"source"`
		Voices      []struct {
			VoiceName       string `json:"voice_name"`
			Label           string `json:"label"`
			Tone            string `json:"tone"`
			Description     string `json:"description"`
			SampleAudioPath string `json:"sample_audio_path"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Provider != "gemini" {
		t.Fatalf("provider = %q, want gemini", resp.Provider)
	}
	if len(resp.Voices) != 30 {
		t.Fatalf("len(voices) = %d, want 30", len(resp.Voices))
	}
	if resp.Voices[0].VoiceName != "Zephyr" {
		t.Fatalf("first voice = %q, want Zephyr", resp.Voices[0].VoiceName)
	}
	if resp.Voices[len(resp.Voices)-1].VoiceName != "Sulafat" {
		t.Fatalf("last voice = %q, want Sulafat", resp.Voices[len(resp.Voices)-1].VoiceName)
	}
	if resp.Voices[0].SampleAudioPath != "/audio/gemini-tts-voices/output_Zephyr.mp3" {
		t.Fatalf("sample_audio_path = %q, want Zephyr sample path", resp.Voices[0].SampleAudioPath)
	}
}
