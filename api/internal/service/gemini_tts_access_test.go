package service

import "testing"

func TestGeminiTTSEnabledForEmail(t *testing.T) {
	t.Setenv("GEMINI_TTS_ALLOWED_EMAILS", "foo@example.com, User-2@Example.com ,bar@example.com")
	if !GeminiTTSEnabledForEmail("user-2@example.com") {
		t.Fatal("GeminiTTSEnabledForEmail(user-2@example.com) = false, want true")
	}
	if GeminiTTSEnabledForEmail("user-x@example.com") {
		t.Fatal("GeminiTTSEnabledForEmail(user-x@example.com) = true, want false")
	}
}
