import os
import json
import subprocess
import sys
import re
from urllib.parse import urlparse, unquote

from app.services.bounded_download import download_bytes
from app.services.url_security import ensure_response_size


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


def _extract_pdf_body_in_process(pdf_bytes: bytes, url: str) -> dict | None:
    import fitz

    if not pdf_bytes:
        return None
    ensure_response_size(pdf_bytes, 25 * 1024 * 1024)

    with fitz.open(stream=pdf_bytes, filetype="pdf") as doc:
        if doc.page_count > 1000:
            raise ValueError("PDF exceeds 1000 pages")
        pages = []
        text_bytes = 0
        for page in doc:
            text = (page.get_text("text") or "").strip()
            text_bytes += len(text.encode("utf-8"))
            if text_bytes > 5 * 1024 * 1024:
                raise ValueError("PDF extracted text exceeds 5MB")
            if text:
                pages.append(text)
        content = _normalize_pdf_text("\n\n".join(pages))
        if not content:
            return None

        metadata = doc.metadata or {}
        title = (metadata.get("title") or "").strip() or _title_from_url(url)
        return {
            "title": (title[:1000] if title else None),
            "content": content,
            "published_at": None,
            "image_url": None,
        }


def extract_pdf_body_from_bytes(pdf_bytes: bytes, url: str) -> dict | None:
    if not pdf_bytes:
        return None
    ensure_response_size(pdf_bytes, 25 * 1024 * 1024)
    # A pathological page can stall native parsing; isolate CPU and memory.
    try:
        result = subprocess.run(
            [sys.executable, "-m", "app.services.pdf_service", url],
            input=pdf_bytes, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
            timeout=20, check=True,
        )
    except (subprocess.TimeoutExpired, subprocess.CalledProcessError) as exc:
        raise ValueError("PDF extraction exceeded limits or failed") from exc
    return json.loads(result.stdout)


def extract_pdf_body(url: str) -> dict | None:
    try:
        content, final_url = download_bytes(url, 25 * 1024 * 1024)
        return extract_pdf_body_from_bytes(content, final_url)
    except Exception:
        if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
            return {
                "title": _title_from_url(url),
                "content": f"[dev placeholder] Failed to extract PDF content for URL: {url}",
                "published_at": None,
                "image_url": None,
            }
        return None


if __name__ == "__main__":
    import resource
    resource.setrlimit(resource.RLIMIT_AS, (512 * 1024 * 1024, 512 * 1024 * 1024))
    resource.setrlimit(resource.RLIMIT_CPU, (15, 15))
    body = sys.stdin.buffer.read(25 * 1024 * 1024 + 1)
    print(json.dumps(_extract_pdf_body_in_process(body, sys.argv[1]), ensure_ascii=False))
