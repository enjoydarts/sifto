import unittest

from app.services.pdf_service import extract_pdf_body_from_bytes


def build_pdf_bytes(text: str | None, title: str | None = None) -> bytes:
    import fitz

    with fitz.open() as doc:
        page = doc.new_page()
        if text:
            page.insert_text((72, 72), text)
        if title:
            doc.set_metadata({"title": title})
        return doc.tobytes()


class PDFServiceTests(unittest.TestCase):
    def test_extract_pdf_body_returns_text(self):
        result = extract_pdf_body_from_bytes(build_pdf_bytes("Hello PDF World", title="PDF title"), "https://example.com/news.pdf")

        self.assertIsNotNone(result)
        assert result is not None
        self.assertEqual(result["title"], "PDF title")
        self.assertIsNone(result["published_at"])
        self.assertIsNone(result["image_url"])
        self.assertIn("Hello PDF World", result["content"])

    def test_extract_pdf_body_returns_none_for_empty_text_pdf(self):
        self.assertIsNone(extract_pdf_body_from_bytes(build_pdf_bytes(None), "https://example.com/empty.pdf"))


if __name__ == "__main__":
    unittest.main()
