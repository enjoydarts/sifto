from unittest.mock import patch
import httpx
import pytest
from app.services.bounded_download import download_bytes


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
