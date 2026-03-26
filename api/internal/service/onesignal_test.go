package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestOneSignalSendToExternalIDIncludesTargetURLInData(t *testing.T) {
	var got map[string]any
	client := &OneSignalClient{
		appID:  "app-id",
		apiKey: "api-key",
		base:   "https://onesignal.test",
		http: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	data, _ := got["data"].(map[string]any)
	if data["target_url"] != "https://app.example.com/audio-briefings/job-1" {
		t.Fatalf("data[target_url] = %v, want target url", data["target_url"])
	}
}
