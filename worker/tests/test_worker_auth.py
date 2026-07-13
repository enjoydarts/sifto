from app.main import _worker_auth_error_status


def test_worker_auth_fails_closed_without_configured_secret():
    assert _worker_auth_error_status("/extract-body", "", "") == 503


def test_worker_auth_requires_matching_secret():
    assert _worker_auth_error_status("/extract-body", "", "secret") == 401
    assert _worker_auth_error_status("/extract-body", "wrong", "secret") == 401
    assert _worker_auth_error_status("/extract-body", "secret", "secret") is None


def test_worker_health_remains_public():
    assert _worker_auth_error_status("/health", "", "") is None
