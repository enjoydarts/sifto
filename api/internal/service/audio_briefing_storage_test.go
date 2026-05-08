package service

import "testing"

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
