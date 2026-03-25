package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNewWorkerClientUsesAudioBriefingTimeout(t *testing.T) {
	t.Setenv("PYTHON_WORKER_AUDIO_BRIEFING_TIMEOUT_SEC", "420")

	client := NewWorkerClient()

	if client.audioBriefingTimeout != 420*time.Second {
		t.Fatalf("audioBriefingTimeout = %v, want %v", client.audioBriefingTimeout, 420*time.Second)
	}
	if client.http == nil {
		t.Fatal("http client is nil")
	}
	if client.http.Timeout != 435*time.Second {
		t.Fatalf("http timeout = %v, want %v", client.http.Timeout, 435*time.Second)
	}
}

func TestSynthesizeAudioBriefingUploadAppliesAudioBriefingTimeout(t *testing.T) {
	t.Setenv("PYTHON_WORKER_AUDIO_BRIEFING_TIMEOUT_SEC", "420")
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Fatal("request deadline was not set")
		}
		remaining := time.Until(deadline)
		if remaining < 418*time.Second || remaining > 421*time.Second {
			t.Fatalf("request deadline remaining = %v, want about 420s", remaining)
		}
		body, _ := json.Marshal(map[string]any{
			"audio_object_key": "foo.mp3",
			"duration_sec":     12,
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})

	_, err := client.SynthesizeAudioBriefingUpload(
		context.Background(),
		"aivis",
		"model",
		"speaker:1",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		0,
		0,
		"foo",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingUpload(...) error = %v", err)
	}
}

func TestSynthesizeAudioBriefingUploadIncludesUserDictionaryUUID(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["user_dictionary_uuid"]; got != "5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861" {
			t.Fatalf("user_dictionary_uuid = %v, want expected uuid", got)
		}
		respBody, _ := json.Marshal(map[string]any{
			"audio_object_key": "foo.mp3",
			"duration_sec":     12,
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	_, err := client.SynthesizeAudioBriefingUpload(
		context.Background(),
		"aivis",
		"model",
		"speaker:1",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		0,
		0,
		"foo",
		strptr("5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861"),
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingUpload(...) error = %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
