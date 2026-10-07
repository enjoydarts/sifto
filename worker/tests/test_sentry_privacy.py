from unittest.mock import patch

import sentry_sdk
from app import main


def test_worker_sentry_never_captures_locals_or_request_bodies():
    with patch.object(main, "_SENTRY_DSN", "https://public@example.com/1"), patch.object(
        sentry_sdk, "init"
    ) as init:
        main._configure_sentry()
    options = init.call_args.kwargs
    assert options.get("include_local_variables") is False
    assert options.get("send_default_pii") is False
    assert options.get("max_request_body_size") == "never"

    event = {
        "event_id": "123",
        "request": {"headers": {"X-Google-Api-Key": "secret"}, "data": "private article"},
        "extra": {"api_key": "secret", "prompt": "private article"},
        "user": {"email": "private@example.com"},
        "breadcrumbs": {"values": [{"message": "private article"}]},
        "logentry": {"message": "private article"},
        "tags": {"api_key": "secret"},
        "contexts": {"custom": {"prompt": "private article"}, "trace": {"trace_id": "abc"}},
        "spans": [{"op": "http.client", "description": "url?api_key=secret", "data": {"prompt": "private article"}}],
        "exception": {"values": [{"type": "ValueError", "value": "private article",
            "stacktrace": {"frames": [{"function": "summarize", "vars": {"api_key": "secret"}}]}}]},
    }
    for hook in ("before_send", "before_send_transaction"):
        scrubbed = options[hook](event, {})
        assert scrubbed["event_id"] == "123"
        assert "secret" not in str(scrubbed)
        assert "private" not in str(scrubbed)
        assert scrubbed["exception"]["values"][0]["type"] == "ValueError"
        assert scrubbed["exception"]["values"][0]["stacktrace"]["frames"][0]["function"] == "summarize"


def test_sdk_sends_only_scrubbed_error_event():
    sent = []
    class CaptureTransport(sentry_sdk.transport.Transport):
        def capture_envelope(self, envelope):
            for item in envelope.items:
                if item.type == "event":
                    sent.append(item.payload.json)
    client = sentry_sdk.Client(
        dsn="https://public@example.com/1",
        default_integrations=False,
        include_local_variables=False,
        send_default_pii=False,
        max_request_body_size="never",
        before_send=main._scrub_sentry_event,
        transport=CaptureTransport,
    )
    try:
        client.capture_event({
            "level": "error",
            "request": {"headers": {"X-Google-Api-Key": "secret"}, "data": "private article"},
            "exception": {"values": [{"type": "ValueError", "value": "private article: secret",
                "stacktrace": {"frames": [{"function": "summarize", "vars": {"api_key": "secret"}}]}}]},
        })
        assert len(sent) == 1
        assert "secret" not in str(sent[0])
        assert "private article" not in str(sent[0])
        assert sent[0]["exception"]["values"][0]["type"] == "ValueError"
    finally:
        client.close()
