import socket
from unittest.mock import patch

import httpx
import pytest

from app.services.trafilatura_service import extract_body
from tests.test_pdf_service import build_pdf_bytes


@pytest.mark.parametrize("pdf_url", ["/start", "/report.pdf"])
def test_redirected_pdf_uses_bounded_fetch_and_real_pdf_parser(pdf_url):
    content = build_pdf_bytes("Hello redirected PDF", title="Redirected document")
    urls = []
    def handler(request):
        urls.append(str(request.url))
        if request.url.path == pdf_url:
            return httpx.Response(302, headers={"location": "/final/document"})
        # Exercise PDF magic-byte detection on the HTML path, too.
        return httpx.Response(200, headers={"content-type": "application/octet-stream"}, stream=httpx.ByteStream(content))
    client = httpx.Client(transport=httpx.MockTransport(handler))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch(
        "app.services.url_security.socket.getaddrinfo", return_value=[(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", 443))]
    ), patch("trafilatura.fetch_url", side_effect=AssertionError("unsafe fetch_url called")):
        result = extract_body("https://example.test" + pdf_url)
    assert result["title"] == "Redirected document"
    assert "Hello redirected PDF" in result["content"]
    assert len(urls) == 2


@pytest.mark.parametrize("encoding", ["utf-8", "cp932", "euc_jp"])
def test_redirected_html_keeps_encoding_and_relative_image(encoding):
    text = "旅先で本文を安全に取得します。" * 30
    html = f'<html><head><title>日本語の記事</title><meta charset="{encoding}"><meta property="og:image" content="cover.jpg"></head><body><article><p>{text}</p></article></body></html>'
    def handler(request):
        if request.url.path == "/start":
            return httpx.Response(302, headers={"location": "/news/article"})
        return httpx.Response(200, headers={"content-type": "text/html"}, stream=httpx.ByteStream(html.encode(encoding)))
    client = httpx.Client(transport=httpx.MockTransport(handler))
    with patch("app.services.bounded_download.httpx.Client", return_value=client), patch(
        "app.services.url_security.socket.getaddrinfo", return_value=[(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", 443))]
    ), patch("trafilatura.fetch_url", side_effect=AssertionError("unsafe fetch_url called")):
        result = extract_body("https://example.test/start")
    assert "旅先で本文を安全に取得します。" in result["content"]
    assert result["title"] == "日本語の記事"
    assert result["image_url"] == "https://example.test/news/cover.jpg"
