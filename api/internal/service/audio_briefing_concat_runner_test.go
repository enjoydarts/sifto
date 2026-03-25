package service

import "testing"

func TestAudioBriefingConcatModeFromEnv(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_CONCAT_MODE", "")
	if got := AudioBriefingConcatModeFromEnv(); got != audioBriefingConcatModeCloudRun {
		t.Fatalf("expected default mode %q, got %q", audioBriefingConcatModeCloudRun, got)
	}

	t.Setenv("AUDIO_BRIEFING_CONCAT_MODE", "local")
	if got := AudioBriefingConcatModeFromEnv(); got != audioBriefingConcatModeLocal {
		t.Fatalf("expected local mode, got %q", got)
	}
}

func TestAudioBriefingCallbackBaseURL(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_LOCAL_CALLBACK_BASE_URL", "")
	if got := audioBriefingCallbackBaseURL(audioBriefingConcatModeLocal); got != defaultAudioBriefingLocalCallbackBaseURL {
		t.Fatalf("expected local callback base %q, got %q", defaultAudioBriefingLocalCallbackBaseURL, got)
	}

	t.Setenv("APP_BASE_URL", "https://example.com/")
	if got := audioBriefingCallbackBaseURL(audioBriefingConcatModeCloudRun); got != "https://example.com" {
		t.Fatalf("expected trimmed app base url, got %q", got)
	}
}
