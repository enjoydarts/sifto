package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enjoydarts/sifto/api/internal/service"
)

func TestResolvePlayableAudioURLKeepsAbsoluteURL(t *testing.T) {
	t.Parallel()

	raw := "https://example.com/audio.mp3"
	got := resolvePlayableAudioURL(context.Background(), nil, "", &raw)
	if got == nil {
		t.Fatalf("expected url, got nil")
	}
	if *got != raw {
		t.Fatalf("expected %q, got %q", raw, *got)
	}
}

func TestResolvePlayableAudioURLPresignsObjectKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/audio-briefing/presign" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := strings.TrimSpace(r.Header.Get("X-Internal-Worker-Secret")); got != "test-secret" {
			t.Fatalf("unexpected worker secret: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), `"bucket":"briefings-ia"`) {
			t.Fatalf("expected bucket in body, got %s", string(body))
		}
		_, _ = w.Write([]byte(`{"audio_url":"https://signed.example.com/audio.mp3"}`))
	}))
	defer server.Close()

	t.Setenv("PYTHON_WORKER_URL", server.URL)
	t.Setenv("INTERNAL_WORKER_SECRET", "test-secret")
	worker := service.NewWorkerClient()
	objectKey := "audio-briefings/user/job/episode.mp3"

	got := resolvePlayableAudioURL(context.Background(), worker, "briefings-ia", &objectKey)
	if got == nil {
		t.Fatalf("expected signed url, got nil")
	}
	if *got != "https://signed.example.com/audio.mp3" {
		t.Fatalf("unexpected signed url: %q", *got)
	}
}
