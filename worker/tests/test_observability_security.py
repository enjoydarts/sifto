from unittest.mock import patch, Mock
from fastapi.testclient import TestClient
import pytest
from app import main
from app.services import langfuse_client
from unittest.mock import AsyncMock
from app.routers import digest
import sentry_sdk


def test_unauthenticated_request_never_creates_trace():
    with patch.object(main, "_INTERNAL_WORKER_SECRET", "test-secret"), patch.object(main, "langfuse_span") as span:
        with TestClient(main.app) as client:
            response = client.post("/extract-body", json={"url": "https://example.com"}, headers={"X-Sifto-User-Id": "victim"})
    assert response.status_code == 401
    span.assert_not_called()


def test_observability_context_entry_failure_does_not_fail_request():
    context = Mock()
    context.__enter__ = Mock(side_effect=RuntimeError("sdk unavailable"))
    context.__exit__ = Mock(return_value=False)
    client = Mock(start_as_current_span=Mock(return_value=context))
    with patch.object(langfuse_client, "_client", return_value=client):
        with langfuse_client.span("request") as span:
            assert span is None


def test_observability_must_not_swallow_application_errors():
    from contextlib import nullcontext
    client = Mock(start_as_current_span=Mock(return_value=nullcontext(Mock())))
    with patch.object(langfuse_client, "_client", return_value=client), pytest.raises(ValueError, match="application"):
        with langfuse_client.span("request"):
            raise ValueError("application failed")

def test_digest_provider_errors_do_not_expose_provider_details():
    with sentry_sdk.isolation_scope() as scope:
        scope.set_client(sentry_sdk.Client(dsn="", default_integrations=False))
        with patch.object(main, "_INTERNAL_WORKER_SECRET", "test-secret"), patch.object(digest, "run_observed_request_async", AsyncMock(side_effect=RuntimeError("private-provider-response"))):
            with TestClient(main.app, raise_server_exceptions=False) as client:
                response = client.post("/compose-digest", json={"digest_date": "2026-10-07", "items": []}, headers={"X-Internal-Worker-Secret": "test-secret"})
        assert response.status_code == 500
        assert "private-provider-response" not in response.text
