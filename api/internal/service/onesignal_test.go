package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type oneSignalRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f oneSignalRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOneSignalSendToExternalIDIncludesTargetURLInData(t *testing.T) {
	t.Setenv("INTERNAL_API_SECRET", strings.Repeat("a", 32))
	var got map[string]any
	client := &OneSignalClient{
		appID:  "app-id",
		apiKey: "api-key",
		base:   "https://onesignal.test",
		http: &http.Client{Transport: oneSignalRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			body, _ := json.Marshal(map[string]any{"id": "notification-1", "recipients": 1})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		})},
	}

	_, err := client.SendToExternalID(
		context.Background(),
		"user@example.com",
		"title",
		"body",
		"https://app.example.com/audio-briefings/job-1",
		map[string]any{"type": "audio_briefing_published"},
	)
	if err != nil {
		t.Fatalf("SendToExternalID(...) error = %v", err)
	}

	if got["url"] != "https://app.example.com/audio-briefings/job-1" {
		t.Fatalf("url = %v, want target url", got["url"])
	}
	aliases := got["include_aliases"].(map[string]any)["external_id"].([]any)
	if aliases[0] == "user@example.com" || !strings.HasPrefix(aliases[0].(string), "sifto_v2_") {
		t.Fatalf("guessable identity: %v", aliases)
	}
	if aliases[0] != "sifto_v2_4386dd05bbea24cdc3e89636ee221cbdfc6fe51d29b4a31348a291f6bab440cf" {
		t.Fatalf("notification identity differs from Web identity: %v", aliases)
	}
	data, _ := got["data"].(map[string]any)
	if data["target_url"] != "https://app.example.com/audio-briefings/job-1" {
		t.Fatalf("data[target_url] = %v, want target url", data["target_url"])
	}
}
