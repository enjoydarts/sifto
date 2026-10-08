import time
import zlib
from urllib.parse import urljoin

import httpx
import httpcore
from app.services.url_security import resolve_public_addresses, validate_public_http_url


class PublicNetworkBackend(httpcore.NetworkBackend):
    """Resolve at connect time, then dial only the validated numeric addresses.

    HTTPCore retains the original hostname for Host, TLS SNI and verification.
    Never patch the process-wide resolver (other worker requests run concurrently).
    """

    def __init__(self):
        self._backend = httpcore.SyncBackend()

    def connect_tcp(self, host, port, timeout=None, local_address=None, socket_options=None):
        addresses = resolve_public_addresses(host, port)
        deadline = None if timeout is None else time.monotonic() + timeout
        for address in addresses:
            remaining = None if deadline is None else deadline - time.monotonic()
            if remaining is not None and remaining <= 0:
                raise httpcore.ConnectTimeout("download connect deadline exceeded")
            try:
                return self._backend.connect_tcp(
                    address, port, timeout=remaining, local_address=local_address,
                    socket_options=socket_options,
                )
            except (httpcore.ConnectError, httpcore.ConnectTimeout):
                if address == addresses[-1]:
                    raise


class PublicHTTPTransport(httpx.HTTPTransport):
    def __init__(self):
        super().__init__(trust_env=False)
        # HTTPX 0.28.1 has no public network_backend constructor argument.
        # Keep its exception mapping/stream adapter, with HTTPCore's backend hook.
        self._pool.close()
        self._pool = httpcore.ConnectionPool(network_backend=PublicNetworkBackend())


def download_response(url: str, max_bytes: int, timeout_sec: float = 30.0) -> httpx.Response:
    """Fetch a bounded body, validating every redirect before connecting."""
    if max_bytes <= 0 or timeout_sec <= 0:
        raise ValueError("download limits must be positive")
    deadline = time.monotonic() + timeout_sec
    with httpx.Client(timeout=timeout_sec, follow_redirects=False, trust_env=False,
                      transport=PublicHTTPTransport(),
                      headers={"Accept-Encoding": "identity"}) as client:
        for _ in range(6):
            url = validate_public_http_url(url)
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("download deadline exceeded")
            with client.stream("GET", url, timeout=min(remaining, 5.0)) as response:
                if response.is_redirect:
                    location = response.headers.get("location")
                    if not location:
                        raise ValueError("download redirect has no location")
                    url = urljoin(str(response.url), location)
                    continue
                response.raise_for_status()
                length = response.headers.get("content-length")
                if length and int(length) > max_bytes:
                    raise ValueError("download exceeds byte limit")
                encoding = response.headers.get("content-encoding", "identity").strip().lower()
                if encoding not in {"identity", "gzip", "deflate"}:
                    raise ValueError("unsupported download content encoding")
                decoder = zlib.decompressobj(31 if encoding == "gzip" else 15) if encoding != "identity" else None
                body = bytearray()
                wire_bytes = 0
                for chunk in response.iter_raw(chunk_size=64 * 1024):
                    if time.monotonic() >= deadline:
                        raise TimeoutError("download deadline exceeded")
                    wire_bytes += len(chunk)
                    if wire_bytes > max_bytes:
                        raise ValueError("download exceeds byte limit")
                    if decoder:
                        chunk = decoder.decompress(chunk, max_bytes - len(body) + 1)
                    if len(body) + len(chunk) > max_bytes:
                        raise ValueError("download exceeds byte limit")
                    body.extend(chunk)
                if decoder and (not decoder.eof or decoder.unused_data):
                    raise ValueError("invalid compressed download")
                headers = dict(response.headers)
                headers.pop("content-encoding", None)
                headers.pop("content-length", None)
                return httpx.Response(response.status_code, headers=headers, content=bytes(body),
                                      request=response.request)
        raise ValueError("too many download redirects")


def download_bytes(url: str, max_bytes: int, timeout_sec: float = 30.0) -> tuple[bytes, str]:
    response = download_response(url, max_bytes, timeout_sec)
    return response.content, str(response.url)
