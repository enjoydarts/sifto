import unittest
from unittest.mock import Mock, patch

from app.services.trafilatura_service import extract_body, is_pdf_response


class TrafilaturaServiceTests(unittest.TestCase):
    def test_is_pdf_response_accepts_content_type(self):
        self.assertTrue(is_pdf_response("https://example.com/file", "application/pdf", b"%PDF-1.7"))

    def test_is_pdf_response_accepts_pdf_extension(self):
        self.assertTrue(is_pdf_response("https://example.com/file.pdf", "application/octet-stream", b""))

    def test_extract_body_uses_pdf_extractor_for_pdf_url(self):
        with patch("app.services.trafilatura_service.extract_pdf_body", return_value={"title": "doc", "content": "text", "published_at": None, "image_url": None}) as mocked_extract:
            result = extract_body("https://example.com/report.pdf")

        self.assertEqual(result["content"], "text")
        mocked_extract.assert_called_once_with("https://example.com/report.pdf")

    def test_extract_body_uses_pdf_bytes_extractor_for_pdf_response_header(self):
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "application/pdf"}
        response.content = b"%PDF-1.7 fake"
        response.url = "https://example.com/final"
        response.text = ""
        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=None), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.extract_pdf_body_from_bytes",
            return_value={"title": "doc", "content": "pdf text", "published_at": None, "image_url": None},
        ) as mocked_extract:
            result = extract_body("https://example.com/start")

        self.assertEqual(result["content"], "pdf text")
        mocked_extract.assert_called_once_with(b"%PDF-1.7 fake", "https://example.com/final")

    def test_extract_body_decodes_shift_jis_html_without_charset_header(self):
        html = "<html><head><title>50歳独身男性のインシデント対応を分析</title></head><body>GoogleがM-Trends 2026公開</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html"}
        response.content = html.encode("cp932")
        response.url = "https://example.com/final"
        response.text = response.content.decode("utf-8", errors="replace")
        captured = {}

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {"title": "50歳独身男性のインシデント対応を分析", "text": "GoogleがM-Trends 2026公開", "date": None}

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=None), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("50歳独身男性のインシデント対応を分析", captured["downloaded"])
        self.assertEqual(result["title"], "50歳独身男性のインシデント対応を分析")
        self.assertEqual(result["content"], "GoogleがM-Trends 2026公開")

    def test_extract_body_refetches_when_fetch_url_result_is_mojibake(self):
        html = "<html><head><title>映画『CUBA JAZZ』始動</title></head><body>キューバの音楽文化を追う</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html"}
        response.content = html.encode("cp932")
        response.url = "https://example.com/final"
        response.text = response.content.decode("utf-8", errors="replace")
        captured = {}

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {"title": "映画『CUBA JAZZ』始動", "text": "キューバの音楽文化を追う", "date": None}

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value="�����T��ē̐V"), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("映画『CUBA JAZZ』始動", captured["downloaded"])
        self.assertEqual(result["title"], "映画『CUBA JAZZ』始動")
        self.assertEqual(result["content"], "キューバの音楽文化を追う")


if __name__ == "__main__":
    unittest.main()
