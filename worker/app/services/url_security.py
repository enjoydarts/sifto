from __future__ import annotations

import ipaddress
import socket
from urllib.parse import urlparse


class UnsafeURLError(ValueError):
    pass


def validate_public_http_url(url: str) -> str:
    normalized = str(url or "").strip()
    parsed = urlparse(normalized)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise UnsafeURLError("URL must use http or https and include a host")
    if parsed.username is not None or parsed.password is not None:
        raise UnsafeURLError("URL credentials are not allowed")
    try:
        addresses = socket.getaddrinfo(parsed.hostname, parsed.port or (443 if parsed.scheme == "https" else 80), type=socket.SOCK_STREAM)
    except OSError as exc:
        raise UnsafeURLError("URL host could not be resolved") from exc
    if not addresses:
        raise UnsafeURLError("URL host has no addresses")
    for address in addresses:
        ip = ipaddress.ip_address(address[4][0])
        if not ip.is_global:
            raise UnsafeURLError("URL resolves to a non-public address")
    return normalized


def ensure_response_size(content: bytes, max_bytes: int) -> None:
    if len(content or b"") > max_bytes:
        raise ValueError(f"response exceeds {max_bytes} bytes")
