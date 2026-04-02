import html
import logging
import os
import re
from cgi import parse_header
from urllib.parse import urljoin

import httpx
import trafilatura
from app.services.pdf_service import extract_pdf_body, extract_pdf_body_from_bytes
from trafilatura.settings import use_config

_log = logging.getLogger(__name__)

_META_IMAGE_PATTERNS = [
    re.compile(r'(?is)<meta[^>]+property=["\']og:image(?::secure_url)?["\'][^>]+content=["\']([^"\']+)["\']'),
    re.compile(r'(?is)<meta[^>]+content=["\']([^"\']+)["\'][^>]+property=["\']og:image(?::secure_url)?["\']'),
    re.compile(r'(?is)<meta[^>]+name=["\']twitter:image(?::src)?["\'][^>]+content=["\']([^"\']+)["\']'),
    re.compile(r'(?is)<meta[^>]+content=["\']([^"\']+)["\'][^>]+name=["\']twitter:image(?::src)?["\']'),
]

_META_CHARSET_PATTERNS = [
    re.compile(br'(?is)<meta[^>]+charset=["\']?\s*([a-zA-Z0-9._\-]+)'),
    re.compile(br'(?is)<meta[^>]+content=["\'][^"\']*charset=\s*([a-zA-Z0-9._\-]+)[^"\']*["\']'),
]
_META_CHARSET_TEXT_PATTERNS = [
    re.compile(r'(?is)<meta[^>]+charset=["\']?\s*([a-zA-Z0-9._\-]+)'),
    re.compile(r'(?is)<meta[^>]+content=["\'][^"\']*charset=\s*([a-zA-Z0-9._\-]+)[^"\']*["\']'),
]
_UTF8_MOJIBAKE_CHARS = set("ƒ‚„…†‡ˆ‰Š‹ŒŽ‘’“”•–—˜™›œžŸپںژگچءإابجىْ؟")
_CJK_UTF8_MOJIBAKE_CHARS = set("丂丄丅乕乮乯乽乿仐仜偁偄偆偉偊偍偐偑偒偓偔偖偗偘偙偛偝偞偟偠偡偣偨偪偫偮偯偰偱偲偳偵偺偼偽偻偼偾傀傚傔傕傫傮傯傰傱傲傴債傶傸傺傼傽傿僀僁僂僃僄僅僆僉僋僌働僐僑僒僓僔僖僗僘僙僚僛僜僝僞僟僠僡僢僣僤僥僦僨僩僪僫僭儊儌儍儎儏儐儑儓儔儕儗儘儞劅惉惗")


def _looks_mojibake(text: str) -> bool:
    sample = (text or "")[:4096]
    if not sample:
        return False
    replacement_count = sample.count("\ufffd")
    return replacement_count >= 3 and replacement_count * 20 >= len(sample)


def _looks_utf8_legacy_mojibake(text: str) -> bool:
    sample = (text or "")[:8192]
    if not sample:
        return False
    suspicious = sum(1 for ch in sample if ch in _UTF8_MOJIBAKE_CHARS)
    if suspicious < 4:
        return False
    non_ascii = sum(1 for ch in sample if ord(ch) > 127)
    if non_ascii == 0:
        return False
    return suspicious * 3 >= non_ascii * 2


def _looks_cjk_utf8_mojibake(text: str) -> bool:
    sample = (text or "")[:8192]
    if not sample:
        return False
    suspicious = sum(1 for ch in sample if ch in _CJK_UTF8_MOJIBAKE_CHARS)
    if suspicious < 6:
        return False
    non_ascii = sum(1 for ch in sample if ord(ch) > 127)
    if non_ascii == 0:
        return False
    return suspicious * 2 >= non_ascii


def _looks_latin_box_utf8_mojibake(text: str) -> bool:
    sample = (text or "")[:8192]
    if not sample:
        return False
    latin_suspicious = sum(1 for ch in sample if 0x00C0 <= ord(ch) <= 0x024F)
    box_suspicious = sum(1 for ch in sample if 0x2500 <= ord(ch) <= 0x259F)
    if latin_suspicious < 8 or box_suspicious < 2:
        return False
    non_ascii = sum(1 for ch in sample if ord(ch) > 127)
    if non_ascii == 0:
        return False
    return (latin_suspicious + box_suspicious) * 4 >= non_ascii * 3


def _declared_charset_in_text(text: str) -> str | None:
    head = (text or "")[:8192]
    for pattern in _META_CHARSET_TEXT_PATTERNS:
        match = pattern.search(head)
        if not match:
            continue
        charset = (match.group(1) or "").strip()
        if charset:
            return charset
    return None


def _needs_refetch(downloaded: str | None) -> bool:
    if not downloaded:
        return True
    if _looks_mojibake(downloaded):
        return True
    declared = (_declared_charset_in_text(downloaded) or "").strip().lower()
    if declared in {"utf-8", "utf8"} and _looks_utf8_legacy_mojibake(downloaded):
        return True
    if declared in {"utf-8", "utf8"} and _looks_cjk_utf8_mojibake(downloaded):
        return True
    if declared in {"utf-8", "utf8"} and _looks_latin_box_utf8_mojibake(downloaded):
        return True
    if declared in {"shift_jis", "shift-jis", "sjis", "cp932", "ms932", "windows-31j"} and "\ufffd" in downloaded:
        return True
    if _looks_utf8_legacy_mojibake(downloaded):
        return True
    if _looks_cjk_utf8_mojibake(downloaded):
        return True
    if _looks_latin_box_utf8_mojibake(downloaded):
        return True
    return False


def _decode_html_response(resp: httpx.Response) -> str:
    content = resp.content or b""
    if not content:
        return resp.text or ""

    declared = _declared_response_encoding(resp.headers.get("content-type"), content)
    candidates = []
    if declared:
        candidates.append(declared)
    candidates.extend(["utf-8", "cp932", "shift_jis", "euc_jp", "iso2022_jp"])

    seen = set()
    for enc in candidates:
        normalized = (enc or "").strip().lower()
        if not normalized or normalized in seen:
            continue
        seen.add(normalized)
        try:
            return content.decode(normalized)
        except Exception:
            continue
    return content.decode("utf-8", errors="replace")


def _refetch_html(url: str) -> tuple[str | None, httpx.Response | None]:
    resp = httpx.get(url, timeout=30.0, follow_redirects=True)
    resp.raise_for_status()
    if is_pdf_response(str(resp.url), resp.headers.get("content-type"), resp.content):
        return None, resp
    return _decode_html_response(resp), resp


def _declared_response_encoding(content_type: str | None, content: bytes) -> str | None:
    if content_type:
        _, params = parse_header(content_type)
        charset = str(params.get("charset") or "").strip()
        if charset:
            return charset
    head = content[:4096]
    for pattern in _META_CHARSET_PATTERNS:
        match = pattern.search(head)
        if not match:
            continue
        charset = match.group(1).decode("ascii", errors="ignore").strip()
        if charset:
            return charset
    return None


def _extract_image_url(downloaded: str, page_url: str) -> str | None:
    for pattern in _META_IMAGE_PATTERNS:
        m = pattern.search(downloaded)
        if not m:
            continue
        raw = html.unescape(m.group(1).strip())
        if not raw or raw.startswith("data:"):
            continue
        return urljoin(page_url, raw)
    return None


def _fallback_extract(downloaded: str, url: str) -> dict | None:
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
        "content": text,
        "published_at": None,
        "image_url": _extract_image_url(downloaded, url),
    }


def _result_value(result, key: str, default=None):
    if result is None:
        return default
    if isinstance(result, dict):
        return result.get(key, default)
    return getattr(result, key, default)


def is_pdf_response(url: str, content_type: str | None, content: bytes | None) -> bool:
    normalized_type = (content_type or "").split(";", 1)[0].strip().lower()
    if normalized_type == "application/pdf":
        return True
    if url.strip().lower().split("?", 1)[0].endswith(".pdf"):
        return True
    if content and content[:5] == b"%PDF-":
        return True
    return False


def extract_body(url: str) -> dict | None:
    try:
        if url.strip().lower().split("?", 1)[0].endswith(".pdf"):
            return extract_pdf_body(url)
        config = use_config()
        config.set("DEFAULT", "EXTRACTION_TIMEOUT", "30")

        downloaded = trafilatura.fetch_url(url)
        if _needs_refetch(downloaded):
            try:
                downloaded, resp = _refetch_html(url)
                if resp is not None and downloaded is None:
                    return extract_pdf_body_from_bytes(resp.content, str(resp.url))
            except Exception as e:
                _log.warning("extract fetch failed url=%s err=%s", url, e)
                if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
                    return {
                        "title": None,
                        "content": f"[dev placeholder] Failed to fetch content for URL: {url}",
                        "published_at": None,
                        "image_url": None,
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
        except Exception as e:
            _log.warning("trafilatura bare_extraction failed url=%s err=%s", url, e)
            result = None

        if result is None:
            fallback = _fallback_extract(downloaded, url)
            if fallback is not None:
                return fallback
            if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
                return {
                    "title": None,
                    "content": f"[dev placeholder] Failed to extract content for URL: {url}",
                    "published_at": None,
                    "image_url": _extract_image_url(downloaded, url),
                }
            return None

        content = _result_value(result, "text", "") or ""
        if not content.strip():
            fallback = _fallback_extract(downloaded, url)
            if fallback is not None:
                return fallback
            if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
                return {
                    "title": _result_value(result, "title"),
                    "content": f"[dev placeholder] Empty extracted content for URL: {url}",
                    "published_at": _result_value(result, "date"),
                    "image_url": _extract_image_url(downloaded, url),
                }

        return {
            "title": _result_value(result, "title"),
            "content": content,
            "published_at": _result_value(result, "date"),
            "image_url": _extract_image_url(downloaded, url),
        }
    except Exception:
        _log.exception("extract_body unexpected failure url=%s", url)
        if os.getenv("ALLOW_DEV_EXTRACT_PLACEHOLDER") == "true":
            return {
                "title": None,
                "content": f"[dev placeholder] Unexpected extract error for URL: {url}",
                "published_at": None,
                "image_url": None,
            }
        return None
