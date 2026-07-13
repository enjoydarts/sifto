import socket
from unittest.mock import patch

import pytest

from app.services.url_security import UnsafeURLError, ensure_response_size, validate_public_http_url


@pytest.mark.parametrize("host", ["127.0.0.1", "10.0.0.1", "169.254.169.254", "::1"])
def test_validate_public_http_url_rejects_non_public_addresses(host):
    address = (host, 80, 0, 0) if ":" in host else (host, 80)
    family = socket.AF_INET6 if ":" in host else socket.AF_INET
    with patch("app.services.url_security.socket.getaddrinfo", return_value=[(family, socket.SOCK_STREAM, 6, "", address)]):
        with pytest.raises(UnsafeURLError):
            validate_public_http_url("http://example.test/")


def test_validate_public_http_url_accepts_public_address():
    with patch(
        "app.services.url_security.socket.getaddrinfo",
        return_value=[(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", 443))],
    ):
        assert validate_public_http_url("https://example.com/path") == "https://example.com/path"


def test_response_size_limit():
    ensure_response_size(b"1234", 4)
    with pytest.raises(ValueError):
        ensure_response_size(b"12345", 4)
