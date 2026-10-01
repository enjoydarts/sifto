import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app import main


@pytest.mark.parametrize(
    ("provided_secret", "expected_detail"),
    [("test-secret", "upstream quota exhausted"), ("", "internal server error")],
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
