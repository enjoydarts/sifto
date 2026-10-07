package service

import "testing"

func TestAudioBriefingChunkHeartbeatURL(t *testing.T) {
	const chunkID = "00000000-0000-4000-8000-000000000001"
	for _, tc := range []struct {
		name, mode, dedicatedBase, localBase, appBase, fallbackBase, wantBase string
	}{
		{"default cloud", "", "", "http://api:8080", "https://api.example.com", "", "https://api.example.com"},
		{"cloud ignores local", "cloud_run", "", "http://api:8080", "https://api.example.com", "", "https://api.example.com"},
		{"local", "local", "", "http://api:8080", "https://api.example.com", "", "http://api:8080"},
		{"local default", "local", "", "", "https://api.example.com", "", "http://api:8080"},
		{"local custom", " LOCAL ", "", "https://internal.example.com/base/", "https://api.example.com", "", "https://internal.example.com/base"},
		{"app fallback", "", "", "http://api:8080", "", "https://fallback.example.com", "https://fallback.example.com"},
		{"dedicated cloud", "cloud_run", " https://heartbeat.example.com/base/ ", "http://api:8080", "https://api.example.com", "", "https://heartbeat.example.com/base"},
		{"dedicated local", "local", "https://heartbeat.example.com", "http://api:8080", "https://api.example.com", "", "https://heartbeat.example.com"},
		{"no cloud base", "", "", "http://api:8080", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AUDIO_BRIEFING_CONCAT_MODE", tc.mode)
			t.Setenv("AUDIO_BRIEFING_HEARTBEAT_BASE_URL", tc.dedicatedBase)
			t.Setenv("AUDIO_BRIEFING_LOCAL_CALLBACK_BASE_URL", tc.localBase)
			t.Setenv("APP_BASE_URL", tc.appBase)
			t.Setenv("NEXT_PUBLIC_APP_URL", tc.fallbackBase)
			want := ""
			if tc.wantBase != "" {
				want = tc.wantBase + "/api/internal/audio-briefings/chunks/" + chunkID + "/heartbeat"
			}
			if got := audioBriefingChunkHeartbeatURL(chunkID); got != want {
				t.Fatalf("heartbeat URL = %q, want %q", got, want)
			}
		})
	}
}
