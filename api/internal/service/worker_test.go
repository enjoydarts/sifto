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
		"",
		"editor",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		"foo",
		"chunk-1",
		"http://api.test/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
		"heartbeat-token",
		nil,
		nil,
		nil,
		nil,
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
		"",
		"editor",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		"foo",
		"chunk-1",
		"http://api.test/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
		"heartbeat-token",
		strptr("5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861"),
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingUpload(...) error = %v", err)
	}
}

func TestSynthesizeAudioBriefingGeminiDuoUploadIncludesTurnsAndGoogleHeader(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/audio-briefing/synthesize-upload-gemini-duo" {
			t.Fatalf("path = %q, want /audio-briefing/synthesize-upload-gemini-duo", req.URL.Path)
		}
		if got := req.Header.Get("X-Google-Api-Key"); got != "google-key" {
			t.Fatalf("X-Google-Api-Key = %q, want google-key", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["host_voice_model"]; got != "Kore" {
			t.Fatalf("host_voice_model = %v, want Kore", got)
		}
		turns, ok := body["turns"].([]any)
		if !ok || len(turns) != 2 {
			t.Fatalf("turns = %#v, want 2 items", body["turns"])
		}
		respBody, _ := json.Marshal(map[string]any{
			"audio_object_key": "foo.wav",
			"duration_sec":     18,
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	resp, err := client.SynthesizeAudioBriefingGeminiDuoUpload(
		context.Background(),
		"gemini-2.5-flash-tts",
		"snark",
		"analyst",
		"Kore",
		"Fenrir",
		"article",
		[]AudioBriefingGeminiDuoTurn{
			{Speaker: "host", Text: "冒頭です"},
			{Speaker: "partner", Text: "補足です"},
		},
		"foo",
		strptr("google-key"),
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingGeminiDuoUpload(...) error = %v", err)
	}
	if resp == nil || resp.AudioObjectKey != "foo.wav" {
		t.Fatalf("response = %#v, want foo.wav", resp)
	}
}

func TestSynthesizeSummaryAudioIncludesUserDictionaryUUID(t *testing.T) {
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
		if got := body["text"]; got != "邦題タイトル\n\n要約本文" {
			t.Fatalf("text = %v, want narration body", got)
		}
		if got := body["chunk_trailing_silence_seconds"]; got != float64(1.0) {
			t.Fatalf("chunk_trailing_silence_seconds = %v, want 1.0", got)
		}
		respBody, _ := json.Marshal(map[string]any{
			"audio_base64":  "Zm9v",
			"content_type":  "audio/mpeg",
			"duration_sec":  12,
			"resolved_text": "邦題タイトル\n\n要約本文",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	resp, err := client.SynthesizeSummaryAudio(
		context.Background(),
		"aivis",
		"model",
		"speaker:1",
		"",
		"邦題タイトル\n\n要約本文",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		strptr("5b6f7aa3-2c34-4ad7-aad0-4e1d683d7861"),
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeSummaryAudio(...) error = %v", err)
	}
	if resp == nil || resp.AudioBase64 != "Zm9v" {
		t.Fatalf("AudioBase64 = %#v, want Zm9v", resp)
	}
}

func TestPreprocessFishSpeechTextIncludesPromptAndProviderHeader(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/fish/preprocess-text" {
			t.Fatalf("path = %q, want /fish/preprocess-text", req.URL.Path)
		}
		if got := req.Header.Get("X-Openai-Api-Key"); got != "openrouter-key" {
			t.Fatalf("X-Openai-Api-Key = %q, want openrouter-key", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["prompt_key"]; got != "fish.summary_preprocess" {
			t.Fatalf("prompt_key = %v, want fish.summary_preprocess", got)
		}
		if got := body["model"]; got != "openrouter::openai/gpt-oss-20b" {
			t.Fatalf("model = %v, want openrouter::openai/gpt-oss-20b", got)
		}
		respBody, _ := json.Marshal(map[string]any{
			"text": "前処理済みテキスト",
			"llm": map[string]any{
				"provider":           "openrouter",
				"model":              "openrouter::openai/gpt-oss-20b",
				"pricing_source":     "openrouter_billed",
				"input_tokens":       10,
				"output_tokens":      20,
				"estimated_cost_usd": 0.001,
			},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	resp, err := client.PreprocessFishSpeechText(
		context.Background(),
		"元テキスト",
		"openrouter::openai/gpt-oss-20b",
		"fish.summary_preprocess",
		strptr("openrouter-key"),
	)
	if err != nil {
		t.Fatalf("PreprocessFishSpeechText(...) error = %v", err)
	}
	if resp == nil || resp.Text != "前処理済みテキスト" {
		t.Fatalf("response = %#v, want text", resp)
	}
}

func TestSynthesizeAudioBriefingUploadIncludesXAIAPIKeyHeader(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Xai-Api-Key"); got != "xai-key" {
			t.Fatalf("X-Xai-Api-Key = %q, want xai-key", got)
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
		"xai",
		"voice-1",
		"",
		"",
		"editor",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		"foo",
		"chunk-1",
		"http://api.test/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
		"heartbeat-token",
		nil,
		nil,
		nil,
		nil,
		strptr("xai-key"),
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingUpload(...) error = %v", err)
	}
}

func TestSynthesizeSummaryAudioIncludesXAIAPIKeyHeader(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Xai-Api-Key"); got != "xai-key" {
			t.Fatalf("X-Xai-Api-Key = %q, want xai-key", got)
		}
		respBody, _ := json.Marshal(map[string]any{
			"audio_base64":  "Zm9v",
			"content_type":  "audio/mpeg",
			"duration_sec":  12,
			"resolved_text": "summary text",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	resp, err := client.SynthesizeSummaryAudio(
		context.Background(),
		"xai",
		"voice-1",
		"",
		"",
		"summary text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		nil,
		nil,
		nil,
		nil,
		strptr("xai-key"),
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeSummaryAudio(...) error = %v", err)
	}
	if resp == nil || resp.AudioBase64 != "Zm9v" {
		t.Fatalf("AudioBase64 = %#v, want Zm9v", resp)
	}
}

func TestSynthesizeSummaryAudioDoesNotIncludeGoogleAPIKeyHeaderForGemini(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Google-Api-Key"); got != "" {
			t.Fatalf("X-Google-Api-Key = %q, want empty", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["tts_model"]; got != "gemini-2.5-flash-tts" {
			t.Fatalf("tts_model = %v, want gemini-2.5-flash-tts", got)
		}
		respBody, _ := json.Marshal(map[string]any{
			"audio_base64":  "Zm9v",
			"content_type":  "audio/mpeg",
			"duration_sec":  12,
			"resolved_text": "summary text",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	resp, err := client.SynthesizeSummaryAudio(
		context.Background(),
		"gemini_tts",
		"Kore",
		"",
		"gemini-2.5-flash-tts",
		"summary text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeSummaryAudio(...) error = %v", err)
	}
	if resp == nil || resp.AudioBase64 != "Zm9v" {
		t.Fatalf("AudioBase64 = %#v, want Zm9v", resp)
	}
}

func TestSynthesizeAudioBriefingUploadIncludesOpenAIAPIKeyHeaderAndTTSModel(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Openai-Api-Key"); got != "openai-key" {
			t.Fatalf("X-Openai-Api-Key = %q, want openai-key", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["tts_model"]; got != "gpt-4o-mini-tts" {
			t.Fatalf("tts_model = %v, want gpt-4o-mini-tts", got)
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
		"openai",
		"alloy",
		"",
		"gpt-4o-mini-tts",
		"editor",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		"foo",
		"chunk-1",
		"http://api.test/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
		"heartbeat-token",
		nil,
		nil,
		nil,
		nil,
		nil,
		strptr("openai-key"),
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingUpload(...) error = %v", err)
	}
}

func TestSynthesizeAudioBriefingUploadDoesNotIncludeGeminiAPIKeyHeaderAndIncludesPersonaAndTTSModel(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Google-Api-Key"); got != "" {
			t.Fatalf("X-Google-Api-Key = %q, want empty", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["tts_model"]; got != "gemini-2.5-flash-tts" {
			t.Fatalf("tts_model = %v, want gemini-2.5-flash-tts", got)
		}
		if got := body["persona"]; got != "editor" {
			t.Fatalf("persona = %v, want editor", got)
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
		"gemini_tts",
		"Kore",
		"",
		"gemini-2.5-flash-tts",
		"editor",
		"text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		"foo",
		"chunk-1",
		"http://api.test/api/internal/audio-briefings/chunks/chunk-1/heartbeat",
		"heartbeat-token",
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SynthesizeAudioBriefingUpload(...) error = %v", err)
	}
}

func TestSynthesizeSummaryAudioIncludesOpenAIAPIKeyHeaderAndTTSModel(t *testing.T) {
	client := NewWorkerClient()
	client.baseURL = "http://worker.test"
	client.http.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Openai-Api-Key"); got != "openai-key" {
			t.Fatalf("X-Openai-Api-Key = %q, want openai-key", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["tts_model"]; got != "gpt-4o-mini-tts" {
			t.Fatalf("tts_model = %v, want gpt-4o-mini-tts", got)
		}
		respBody, _ := json.Marshal(map[string]any{
			"audio_base64":  "Zm9v",
			"content_type":  "audio/mpeg",
			"duration_sec":  12,
			"resolved_text": "summary text",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	resp, err := client.SynthesizeSummaryAudio(
		context.Background(),
		"openai",
		"alloy",
		"",
		"gpt-4o-mini-tts",
		"summary text",
		1.0,
		1.0,
		1.0,
		0.4,
		1.0,
		0,
		0,
		nil,
		nil,
		nil,
		nil,
		nil,
		strptr("openai-key"),
	)
	if err != nil {
		t.Fatalf("SynthesizeSummaryAudio(...) error = %v", err)
	}
	if resp == nil || resp.AudioBase64 != "Zm9v" {
		t.Fatalf("AudioBase64 = %#v, want Zm9v", resp)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
