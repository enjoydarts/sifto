package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestScrubSentryEventRemovesPrivateRequestAndErrorData(t *testing.T) {
	event := &sentry.Event{
		EventID:     "123",
		Request:     &sentry.Request{Data: "private article", QueryString: "api_key=secret", Headers: map[string]string{"X-Google-Api-Key": "secret"}},
		User:        sentry.User{Email: "private@example.com"},
		Message:     "private article",
		Breadcrumbs: []*sentry.Breadcrumb{{Message: "private article"}},
		Exception:   []sentry.Exception{{Type: "error", Value: "secret", Stacktrace: &sentry.Stacktrace{Frames: []sentry.Frame{{Function: "summarize", Vars: map[string]interface{}{"api_key": "secret"}}}}}},
	}
	// Extra was removed in Sentry 0.49. Decode the wire field so the same
	// regression exercises its removal on older SDKs and compiles on both.
	if err := json.Unmarshal([]byte(`{"extra":{"api_key":"secret-extra"}}`), event); err != nil {
		t.Fatal(err)
	}
	out := ScrubSentryEvent(event, nil)
	body, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), "private") {
		t.Fatalf("sensitive data remains in event: %s", body)
	}
	if out.EventID != "123" || out.Exception[0].Type != "error" || out.Exception[0].Stacktrace.Frames[0].Function != "summarize" {
		t.Fatal("diagnostic metadata was removed")
	}
}

func TestScrubSentryEventRemovesLegacyExtraFromSerializedPayload(t *testing.T) {
	var event sentry.Event
	if err := json.Unmarshal([]byte(`{"event_id":"123","extra":{"article":"private-extra"}}`), &event); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ScrubSentryEvent(&event, nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "private-extra") || strings.Contains(string(body), `"extra"`) {
		t.Fatalf("legacy extra remains in event: %s", body)
	}
	if event.EventID != "123" {
		t.Fatal("event identity was removed")
	}
}

func TestScrubSentryEventAcceptsNil(t *testing.T) {
	if out := ScrubSentryEvent(nil, nil); out != nil {
		t.Fatal("nil event should remain nil")
	}
}
