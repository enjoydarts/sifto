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

    def test_extract_body_refetches_when_shift_jis_page_contains_sparse_mojibake(self):
        html = "<html><head><title>高橋慎一監督の新作映画『ハバナの奇跡』</title></head><body>社会主義国でのジャズクラブ誕生を追う</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html"}
        response.content = html.encode("cp932")
        response.url = "https://example.com/final"
        response.text = response.content.decode("utf-8", errors="replace")
        captured = {}
        fetched = "<html><head>" + ("a" * 3900) + '<meta charset="Shift_JIS" />' + "�����T��ē̐V" + "</head></html>"

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {"title": "高橋慎一監督の新作映画『ハバナの奇跡』", "text": "社会主義国でのジャズクラブ誕生を追う", "date": None}

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=fetched), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("高橋慎一監督の新作映画『ハバナの奇跡』", captured["downloaded"])
        self.assertEqual(result["title"], "高橋慎一監督の新作映画『ハバナの奇跡』")
        self.assertEqual(result["content"], "社会主義国でのジャズクラブ誕生を追う")

    def test_extract_body_refetches_when_utf8_page_is_decoded_as_legacy_japanese_encoding(self):
        html = "<html><head><meta charset=\"utf-8\"><title>涼宮ハルヒの憂鬱「DEATH NOTE」の放送20周年アニメ7作品、ABEMAで一挙無料配信</title></head><body>ABEMAが周年アニメ特集を始める。</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html; charset=utf-8"}
        response.content = html.encode("utf-8")
        response.url = "https://example.com/final"
        response.text = html
        captured = {}
        fetched = "<html><head>" + ("a" * 3900) + '<meta charset="utf-8" />' + "w—ء‹{ƒnƒ‹ƒq‚ج—JںT" + "</head></html>"

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {
                "title": "涼宮ハルヒの憂鬱「DEATH NOTE」の放送20周年アニメ7作品、ABEMAで一挙無料配信",
                "text": "ABEMAが周年アニメ特集を始める。",
                "date": None,
            }

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=fetched), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("涼宮ハルヒの憂鬱", captured["downloaded"])
        self.assertEqual(result["title"], "涼宮ハルヒの憂鬱「DEATH NOTE」の放送20周年アニメ7作品、ABEMAで一挙無料配信")
        self.assertEqual(result["content"], "ABEMAが周年アニメ特集を始める。")

    def test_extract_body_refetches_when_utf8_page_is_decoded_as_cjk_mojibake(self):
        html = "<html><head><meta charset=\"utf-8\"><title>「なんか記事文字化けする」入力からAIが2ちゃんねる風UIでレス生成するシミュレーターが登場</title></head><body>スレタイと最初のコメントを入力するとAIがレスを生成する。</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html; charset=utf-8"}
        response.content = html.encode("utf-8")
        response.url = "https://example.com/final"
        response.text = html
        captured = {}
        fetched = (
            "<html><head>"
            + ("a" * 3900)
            + '<meta charset="utf-8" />'
            + "丂乽仜仜偩偗偳幙栤偁傞丠乿乽仜仜偟偨傗偮偑桪彑乿偲偄偭偨僗儗僞僀乮尒弌偟乯偲丄嵟弶偺僐儊儞僩"
            + "</head></html>"
        )

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {
                "title": "「なんか記事文字化けする」入力からAIが2ちゃんねる風UIでレス生成するシミュレーターが登場",
                "text": "スレタイと最初のコメントを入力するとAIがレスを生成する。",
                "date": None,
            }

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=fetched), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("なんか記事文字化けする", captured["downloaded"])
        self.assertEqual(result["title"], "「なんか記事文字化けする」入力からAIが2ちゃんねる風UIでレス生成するシミュレーターが登場")
        self.assertEqual(result["content"], "スレタイと最初のコメントを入力するとAIがレスを生成する。")

    def test_extract_body_refetches_when_utf8_page_is_decoded_as_latin_box_mojibake(self):
        html = "<html><head><meta charset=\"utf-8\"><title>NASAは4年1か月ぶりに有人月探査へ向けたロケット『SLS』の打ち上げを目指す。</title></head><body>アルテミス計画の進捗をまとめる。</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html; charset=utf-8"}
        response.content = html.encode("utf-8")
        response.url = "https://example.com/final"
        response.text = html
        captured = {}
        fetched = (
            "<html><head>"
            + ("a" * 3900)
            + '<meta charset="utf-8" />'
            + "ü@Ľ─NASAé═4îÄ1ô˙üiî╗ĺnÄ×ŐďüjüAŚLÉlëFĺłĹDüuâIâŐâIâôüvéôőŹ┌éÁéŻĹňî^âŹâPâbâgüuSLSüvé╠Ĺ┼é┐ĆŃé░é╔ÉČî¸éÁéŻüB"
            + "</head></html>"
        )

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {
                "title": "NASAは4年1か月ぶりに有人月探査へ向けたロケット『SLS』の打ち上げを目指す。",
                "text": "アルテミス計画の進捗をまとめる。",
                "date": None,
            }

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=fetched), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("NASAは4年1か月ぶり", captured["downloaded"])
        self.assertEqual(result["title"], "NASAは4年1か月ぶりに有人月探査へ向けたロケット『SLS』の打ち上げを目指す。")
        self.assertEqual(result["content"], "アルテミス計画の進捗をまとめる。")

    def test_extract_body_prefers_utf8_when_declared_shift_jis_result_is_cjk_mojibake(self):
        html = "<html><head><title>TOPPAN、ギリシャ語写本の本文を解読できるAI-OCRを開発</title></head><body>ギリシャ語写本の本文を解読できるAI-OCRを開発したと発表した。</body></html>"
        response = Mock()
        response.raise_for_status.return_value = None
        response.headers = {"content-type": "text/html; charset=Shift_JIS"}
        response.content = html.encode("utf-8")
        response.url = "https://example.com/final"
        response.text = response.content.decode("cp932", errors="ignore")
        captured = {}

        def fake_bare_extraction(downloaded, **kwargs):
            captured["downloaded"] = downloaded
            return {
                "title": "TOPPAN、ギリシャ語写本の本文を解読できるAI-OCRを開発",
                "text": "ギリシャ語写本の本文を解読できるAI-OCRを開発したと発表した。",
                "date": None,
            }

        with patch("app.services.trafilatura_service.trafilatura.fetch_url", return_value=None), patch(
            "app.services.trafilatura_service.httpx.get", return_value=response
        ), patch(
            "app.services.trafilatura_service.trafilatura.bare_extraction",
            side_effect=fake_bare_extraction,
        ):
            result = extract_body("https://example.com/start")

        self.assertIn("TOPPAN、ギリシャ語写本", captured["downloaded"])
        self.assertEqual(result["title"], "TOPPAN、ギリシャ語写本の本文を解読できるAI-OCRを開発")
        self.assertEqual(result["content"], "ギリシャ語写本の本文を解読できるAI-OCRを開発したと発表した。")


if __name__ == "__main__":
    unittest.main()
