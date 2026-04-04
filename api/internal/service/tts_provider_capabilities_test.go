package service

import "testing"

func TestLookupTTSProviderCapabilitiesIncludesGeminiTTS(t *testing.T) {
	caps := LookupTTSProviderCapabilities("gemini_tts")

	if !caps.SupportsCatalogPicker {
		t.Fatalf("SupportsCatalogPicker = false, want true")
	}
	if !caps.SupportsSeparateTTSModel {
		t.Fatalf("SupportsSeparateTTSModel = false, want true")
	}
	if caps.RequiresVoiceStyle {
		t.Fatalf("RequiresVoiceStyle = true, want false")
	}
	if caps.RequiresUserAPIKey {
		t.Fatalf("RequiresUserAPIKey = true, want false")
	}
}
