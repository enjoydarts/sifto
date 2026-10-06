package service

import (
	"testing"
	"time"
)

func TestPodcastExpiryUsesPodcastRetentionInsteadOfLegacyIAMoveSetting(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_IA_MOVE_AFTER_DAYS", "1")
	t.Setenv("PODCAST_EPISODE_RETENTION_DAYS", "14")
	published := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	got := AudioBriefingPodcastExpiresAt(&published)
	if got == nil || !got.Equal(published.AddDate(0, 0, 14)) {
		t.Fatalf("podcast expires at %v, want 14 days after publication", got)
	}
}

func TestAudioBriefingPodcastPublicObjectURLUsesPublicBaseURL(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_PUBLIC_BUCKET", "briefings-public")
	t.Setenv("AUDIO_BRIEFING_PUBLIC_BASE_URL", "https://media.example.com/audio")

	got := AudioBriefingPodcastPublicObjectURL("briefings-public", "/audio-briefings/job-1.mp3")

	if got == nil {
		t.Fatal("AudioBriefingPodcastPublicObjectURL() = nil, want URL")
	}
	if want := "https://media.example.com/audio/audio-briefings/job-1.mp3"; *got != want {
		t.Fatalf("AudioBriefingPodcastPublicObjectURL() = %q, want %q", *got, want)
	}
}

func TestAudioBriefingPodcastPublicObjectURLRejectsNonPublicBucket(t *testing.T) {
	t.Setenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET", "briefings-standard")
	t.Setenv("AUDIO_BRIEFING_PUBLIC_BUCKET", "briefings-public")
	t.Setenv("AUDIO_BRIEFING_PUBLIC_BASE_URL", "https://media.example.com/audio")

	got := AudioBriefingPodcastPublicObjectURL("briefings-standard", "audio-briefings/job-1.mp3")

	if got != nil {
		t.Fatalf("AudioBriefingPodcastPublicObjectURL() = %q, want nil", *got)
	}
}
