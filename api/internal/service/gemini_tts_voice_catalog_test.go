package service

import (
	"context"
	"testing"
)

func TestGeminiTTSVoiceCatalogServiceLoadCatalog(t *testing.T) {
	svc := NewGeminiTTSVoiceCatalogService()

	catalog, err := svc.LoadCatalog(context.Background())
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if catalog == nil {
		t.Fatal("LoadCatalog() returned nil catalog")
	}
	if catalog.Provider != "gemini" {
		t.Fatalf("Provider = %q, want gemini", catalog.Provider)
	}
	if len(catalog.Voices) != 30 {
		t.Fatalf("len(Voices) = %d, want 30", len(catalog.Voices))
	}
	if catalog.Voices[0].VoiceName != "Zephyr" || catalog.Voices[len(catalog.Voices)-1].VoiceName != "Sulafat" {
		t.Fatalf("Voices = %#v, want curated Gemini voice order", catalog.Voices)
	}
	if catalog.Voices[0].SampleAudioPath != "/audio/gemini-tts-voices/output_Zephyr.mp3" {
		t.Fatalf("SampleAudioPath = %q, want Zephyr sample path", catalog.Voices[0].SampleAudioPath)
	}
}
