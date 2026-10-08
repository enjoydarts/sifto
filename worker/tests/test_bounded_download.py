from unittest.mock import patch
import httpx
import pytest
from app.services.bounded_download import download_bytes
from app.services.bounded_download import PublicNetworkBackend, PublicHTTPTransport
from app.services.url_security import UnsafeURLError, validate_public_http_url
import socket
import gzip
import httpcore
from unittest.mock import Mock


def addresses(*ips):
    return [(socket.AF_INET6 if ":" in ip else socket.AF_INET, socket.SOCK_STREAM, 6, "", (ip, 443)) for ip in ips]


@pytest.mark.parametrize("ip", ["127.0.0.1", "10.1.2.3", "169.254.169.254", "::1", "fc00::1", "::ffff:127.0.0.1"])
def test_dns_rebinding_is_rejected_before_socket_connect(ip):
    backend = PublicNetworkBackend()
    backend._backend = Mock()
    with patch("app.services.url_security.socket.getaddrinfo", side_effect=[addresses("93.184.216.34"), addresses(ip)]):
        validate_public_http_url("https://example.test/")
        with pytest.raises(UnsafeURLError):
            backend.connect_tcp("example.test", 443)
    backend._backend.connect_tcp.assert_not_called()


def test_mixed_dns_answers_are_rejected_before_connect():
    backend = PublicNetworkBackend()
    backend._backend = Mock()
    with patch("app.services.url_security.socket.getaddrinfo", return_value=addresses("93.184.216.34", "10.0.0.1")):
        with pytest.raises(UnsafeURLError):
            backend.connect_tcp("example.test", 443)
    backend._backend.connect_tcp.assert_not_called()


def test_numeric_connect_preserves_host_and_tls_verification():
    class Stream(httpcore.NetworkStream):
        def __init__(self):
            self.writes = []
            self.sni = None
        def write(self, buffer, timeout=None):
            self.writes.append(buffer)
        def read(self, max_bytes, timeout=None):
            return b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok"
        def start_tls(self, ssl_context, server_hostname=None, timeout=None):
            assert ssl_context.check_hostname
            self.sni = server_hostname
            return self
        def close(self):
            pass
    stream = Stream()
    with patch("app.services.url_security.socket.getaddrinfo", return_value=addresses("93.184.216.34")), patch(
        "httpcore.SyncBackend.connect_tcp", return_value=stream
    ) as connect:
        with httpx.Client(transport=PublicHTTPTransport(), trust_env=False) as client:
            assert client.get("https://example.test/article").text == "ok"
    assert connect.call_args.args[:2] == ("93.184.216.34", 443)
    assert stream.sni == "example.test"
    assert b"Host: example.test" in b"".join(stream.writes)


def mock_download(handler, limit=1024):
    client = httpx.Client(transport=httpx.MockTransport(handler))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch(
        "app.services.bounded_download.validate_public_http_url", side_effect=lambda url: url
    ):
        return download_bytes("https://example.test/start", limit)


def test_relative_redirect_returns_final_url_and_body():
    urls = []
    def handler(request):
        urls.append(str(request.url))
        if request.url.path == "/start":
            return httpx.Response(302, headers={"location": "/article"})
        return httpx.Response(200, stream=httpx.ByteStream(b"hello"))
    assert mock_download(handler) == (b"hello", "https://example.test/article")
    assert len(urls) == 2


def test_redirect_loop_is_bounded():
    urls = []
    def handler(request):
        urls.append(str(request.url))
        return httpx.Response(302, headers={"location": "/start"})
    with pytest.raises(ValueError, match="too many"):
        mock_download(handler)
    assert len(urls) == 6


def test_content_length_rejected_without_reading_body():
    class Stream(httpx.SyncByteStream):
        def __iter__(self):
            raise AssertionError("body must not be read")
            yield b""
    with pytest.raises(ValueError, match="byte limit"):
        mock_download(lambda request: httpx.Response(200, headers={"content-length": "1025"}, stream=Stream()))


def test_gzip_expansion_is_bounded():
    payload = gzip.compress(b"x" * 1000000)
    with pytest.raises(ValueError, match="byte limit"):
        mock_download(lambda request: httpx.Response(200, headers={"content-encoding": "gzip"}, stream=httpx.ByteStream(payload)), 2048)


def test_gzip_under_limit_is_decoded_once():
    payload = gzip.compress(b"hello")
    assert mock_download(lambda request: httpx.Response(200, headers={"content-encoding": "gzip"}, stream=httpx.ByteStream(payload)))[0] == b"hello"


@pytest.mark.parametrize("target", ["http://127.0.0.1/", "http://169.254.169.254/", "http://[::1]/", "file:///etc/passwd", "http://user:pass@example.test/"])
def test_redirect_target_policy_is_enforced_before_request(target):
    requests = []
    def handler(request):
        requests.append(str(request.url))
        return httpx.Response(302, headers={"location": target})
    def resolve(host, port, **kwargs):
        return addresses("93.184.216.34" if host == "example.test" else host)
    client = httpx.Client(transport=httpx.MockTransport(handler))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch(
        "app.services.url_security.socket.getaddrinfo", side_effect=resolve
    ):
        with pytest.raises(UnsafeURLError):
            download_bytes("https://example.test/start", 1024)
    assert requests == ["https://example.test/start"]


def test_download_deadline_is_checked_while_reading():
    class Stream(httpx.SyncByteStream):
        def __iter__(self):
            yield b"hello"
    client = httpx.Client(transport=httpx.MockTransport(lambda request: httpx.Response(200, stream=Stream())))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch(
        "app.services.bounded_download.validate_public_http_url", side_effect=lambda url: url
    ), patch("app.services.bounded_download.time.monotonic", side_effect=[0, 0, 31]):
        with pytest.raises(TimeoutError, match="deadline"):
            download_bytes("https://example.test", 1024)


def test_environment_proxy_is_disabled():
    with patch.dict("os.environ", {"HTTP_PROXY": "http://127.0.0.1:8080", "HTTPS_PROXY": "http://127.0.0.1:8080"}):
        with httpx.Client(transport=PublicHTTPTransport(), trust_env=False) as client:
            assert not client._mounts


def test_stream_stops_before_reading_entire_oversized_response():
    read = []
    class Stream(httpx.SyncByteStream):
        def __iter__(self):
            for i in range(100):
                read.append(i)
                yield b"x" * 65536
    client = httpx.Client(transport=httpx.MockTransport(lambda request: httpx.Response(200, stream=Stream())))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch("app.services.bounded_download.validate_public_http_url", side_effect=lambda url: url):
        with pytest.raises(ValueError, match="byte limit"):
            download_bytes("https://example.com", 65536)
    assert len(read) == 2


def test_redirect_is_validated_before_following():
    urls = []
    def handler(request):
        urls.append(str(request.url))
        return httpx.Response(302, headers={"location": "http://127.0.0.1/private"})
    def validate(url):
        if "127.0.0.1" in url:
            raise ValueError("private URL")
        return url
    client = httpx.Client(transport=httpx.MockTransport(handler))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch("app.services.bounded_download.validate_public_http_url", side_effect=validate):
        with pytest.raises(ValueError, match="private URL"):
            download_bytes("https://example.com", 1024)
    assert urls == ["https://example.com"]
