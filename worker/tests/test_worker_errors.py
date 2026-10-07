import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app import main


@pytest.mark.parametrize(
    ("provided_secret", "expected_detail"),
    [("test-secret", "internal server error"), ("", "internal server error")],
)
def test_unhandled_error_closes_connection_before_fallback(
    monkeypatch, provided_secret, expected_detail
):
    monkeypatch.setattr(main, "_INTERNAL_WORKER_SECRET", "test-secret")
    app = FastAPI()
    app.add_exception_handler(Exception, main.global_exception_handler)

    @app.post("/extract-facts")
    async def fail():
        raise RuntimeError("upstream quota exhausted")

    with TestClient(app, raise_server_exceptions=False) as client:
        response = client.post(
            "/extract-facts",
            headers={"X-Internal-Worker-Secret": provided_secret},
        )

    assert response.status_code == 500
    assert response.json() == {"detail": expected_detail}
    assert response.headers.get("connection") == "close"

@pytest.mark.parametrize(("message", "expected"), [
    ("facts check short_comment missing: parse failed: private-output", "LLM response parse failed"),
    ("provider status=429 body=private-output", "upstream rate limit"),
    ("provider timeout api_key=private-key", "upstream timeout"),
])
def test_public_error_keeps_only_fixed_fallback_category(monkeypatch, message, expected):
    from starlette.requests import Request
    monkeypatch.setattr(main, "_INTERNAL_WORKER_SECRET", "test-secret")
    request = Request({"type": "http", "headers": [(b"x-internal-worker-secret", b"test-secret")]})
    assert main._public_error_detail(request, RuntimeError(message)) == expected
