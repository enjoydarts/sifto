import html
import os
import re

import httpx
import trafilatura
from trafilatura.settings import use_config


def _fallback_extract(downloaded: str) -> dict | None:
    title_match = re.search(r"<title[^>]*>(.*?)</title>", downloaded, flags=re.IGNORECASE | re.DOTALL)
    title = None
    if title_match:
        title = html.unescape(re.sub(r"\s+", " ", title_match.group(1)).strip())

    text = re.sub(r"(?is)<script.*?>.*?</script>", " ", downloaded)
    text = re.sub(r"(?is)<style.*?>.*?</style>", " ", text)
    text = re.sub(r"(?s)<[^>]+>", " ", text)
    text = html.unescape(re.sub(r"\s+", " ", text)).strip()
    if not text:
        return None

    return {
        "title": title,
        "content": text[:20000],
        "published_at": None,
    }


def _result_value(result, key: str, default=None):
    if result is None:
        return default
    if isinstance(result, dict):
        return result.get(key, default)
    return getattr(result, key, default)


def extract_body(url: str) -> dict | None:
    config = use_config()
    config.set("DEFAULT", "EXTRACTION_TIMEOUT", "30")

    downloaded = trafilatura.fetch_url(url)
    if downloaded is None:
        try:
            resp = httpx.get(url, timeout=30.0, follow_redirects=True)
            resp.raise_for_status()
            downloaded = resp.text
        except Exception:
            if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
                return {
                    "title": None,
                    "content": f"[dev placeholder] Failed to fetch content for URL: {url}",
                    "published_at": None,
                }
            return None

    try:
        # `output_format="python"` is only supported by bare_extraction().
        result = trafilatura.bare_extraction(
            downloaded,
            include_comments=False,
            include_tables=False,
            with_metadata=True,
            config=config,
        )
    except Exception:
        result = None
    if result is None:
        fallback = _fallback_extract(downloaded)
        if fallback is not None:
            return fallback
        if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
            return {
                "title": None,
                "content": f"[dev placeholder] Failed to extract content for URL: {url}",
                "published_at": None,
            }
        return None

    content = _result_value(result, "text", "") or ""
    if not content.strip():
        fallback = _fallback_extract(downloaded)
        if fallback is not None:
            return fallback
        if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
            return {
                "title": _result_value(result, "title"),
                "content": f"[dev placeholder] Empty extracted content for URL: {url}",
                "published_at": _result_value(result, "date"),
            }

    return {
        "title": _result_value(result, "title"),
        "content": content,
        "published_at": _result_value(result, "date"),
    }
