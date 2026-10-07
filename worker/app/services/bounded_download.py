import time
from urllib.parse import urljoin

import httpx
from app.services.url_security import validate_public_http_url


def download_bytes(url: str, max_bytes: int, timeout_sec: float = 30.0) -> tuple[bytes, str]:
    deadline = time.monotonic() + timeout_sec
    with httpx.Client(timeout=timeout_sec, follow_redirects=False) as client:
        for _ in range(6):
            url = validate_public_http_url(url)
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("download deadline exceeded")
            with client.stream("GET", url, timeout=min(remaining, 5.0)) as response:
                if response.is_redirect:
                    url = urljoin(url, response.headers.get("location", ""))
                    continue
                response.raise_for_status()
                length = response.headers.get("content-length")
                if length and int(length) > max_bytes:
                    raise ValueError("download exceeds byte limit")
                body = bytearray()
                for chunk in response.iter_bytes(chunk_size=64 * 1024):
                    if time.monotonic() >= deadline:
                        raise TimeoutError("download deadline exceeded")
                    if len(body) + len(chunk) > max_bytes:
                        raise ValueError("download exceeds byte limit")
                    body.extend(chunk)
                return bytes(body), str(response.url)
        raise ValueError("too many download redirects")
