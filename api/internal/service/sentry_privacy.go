package service

import "github.com/getsentry/sentry-go"

// ScrubSentryEvent retains exception types and stack locations, without sending
// user requests, provider error text, or logging payloads to telemetry.
func ScrubSentryEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}
	event.Request = nil
	event.User = sentry.User{}
	event.Extra = nil
	event.Breadcrumbs = nil
	event.Message = ""
	event.Tags = nil
	event.Fingerprint = nil
	event.Attachments = nil
	trace := event.Contexts["trace"]
	safeTrace := sentry.Context{}
	for _, key := range []string{"trace_id", "span_id", "parent_span_id", "op", "status"} {
		if value, ok := trace[key]; ok {
			safeTrace[key] = value
		}
	}
	event.Contexts = map[string]sentry.Context{"trace": safeTrace}
	for _, span := range event.Spans {
		if span != nil {
			span.Data = nil
			span.Description = ""
			span.Tags = nil
		}
	}
	for i := range event.Exception {
		event.Exception[i].Value = "[Filtered]"
		scrubSentryStacktrace(event.Exception[i].Stacktrace)
	}
	for i := range event.Threads {
		scrubSentryStacktrace(event.Threads[i].Stacktrace)
	}
	return event
}

func scrubSentryStacktrace(stack *sentry.Stacktrace) {
	if stack == nil {
		return
	}
	for i := range stack.Frames {
		stack.Frames[i].Vars = nil
	}
}
