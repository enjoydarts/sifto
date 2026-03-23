import os
import re
from urllib.parse import urlparse, unquote

import httpx


def _normalize_pdf_text(text: str) -> str:
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = re.sub(r"[ \t]+\n", "\n", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def _title_from_url(url: str) -> str | None:
    path = unquote(urlparse(url).path or "").strip()
    if not path:
        return None
    filename = path.rsplit("/", 1)[-1].strip()
    if not filename:
        return None
    if filename.lower().endswith(".pdf"):
        filename = filename[:-4]
    filename = filename.strip()
    return filename or None


def extract_pdf_body_from_bytes(pdf_bytes: bytes, url: str) -> dict | None:
    import fitz

    if not pdf_bytes:
        return None

    with fitz.open(stream=pdf_bytes, filetype="pdf") as doc:
        pages = []
        for page in doc:
            text = (page.get_text("text") or "").strip()
            if text:
                pages.append(text)
        content = _normalize_pdf_text("\n\n".join(pages))
        if not content:
            return None

        metadata = doc.metadata or {}
        title = (metadata.get("title") or "").strip() or _title_from_url(url)
        return {
            "title": title or None,
            "content": content,
            "published_at": None,
            "image_url": None,
        }


def extract_pdf_body(url: str) -> dict | None:
    try:
        resp = httpx.get(url, timeout=30.0, follow_redirects=True)
        resp.raise_for_status()
        return extract_pdf_body_from_bytes(resp.content, str(resp.url))
    except Exception:
        if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
            return {
                "title": _title_from_url(url),
                "content": f"[dev placeholder] Failed to extract PDF content for URL: {url}",
                "published_at": None,
                "image_url": None,
            }
        return None
