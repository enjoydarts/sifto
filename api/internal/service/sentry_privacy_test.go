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
		Extra:       map[string]interface{}{"api_key": "secret"},
		Message:     "private article",
		Breadcrumbs: []*sentry.Breadcrumb{{Message: "private article"}},
		Exception:   []sentry.Exception{{Type: "error", Value: "secret", Stacktrace: &sentry.Stacktrace{Frames: []sentry.Frame{{Function: "summarize", Vars: map[string]interface{}{"api_key": "secret"}}}}}},
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
