from __future__ import annotations

import json
import re
import subprocess
from urllib.parse import parse_qs, urlparse

import httpx

_YOUTUBE_HOSTS = {
    "youtube.com",
    "www.youtube.com",
    "m.youtube.com",
    "youtu.be",
    "www.youtu.be",
}

_LANGUAGE_PREFERENCE = [
    "ja",
    "ja-jp",
    "en",
    "en-us",
]

_FORMAT_PREFERENCE = ["json3", "vtt", "srv3", "srv2", "srv1", "ttml"]


def is_youtube_url(url: str) -> bool:
    parsed = urlparse((url or "").strip())
    host = (parsed.netloc or "").strip().lower()
    if host not in _YOUTUBE_HOSTS:
        return False
    path = (parsed.path or "").strip()
    if host.endswith("youtu.be"):
        return path not in {"", "/"}
    if path == "/watch" and parse_qs(parsed.query).get("v"):
        return True
    return path.startswith("/shorts/") or path.startswith("/live/")


def extract_body(url: str) -> dict | None:
    metadata = _load_video_metadata(url)
    title = str(metadata.get("title") or "").strip()
    if not title:
        raise RuntimeError("youtube metadata unavailable")

    transcript = _extract_transcript(metadata)
    if not transcript:
        raise RuntimeError("youtube transcript unavailable")

    published_at = _normalize_upload_date(str(metadata.get("upload_date") or "").strip())
    image_url = str(metadata.get("thumbnail") or "").strip() or None
    return {
        "title": title,
        "content": transcript,
        "published_at": published_at,
        "image_url": image_url,
    }


def _load_video_metadata(url: str) -> dict:
    cmd = ["yt-dlp", "--dump-single-json", "--no-warnings", "--skip-download", url]
    proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
    payload = json.loads(proc.stdout or "{}")
    if not isinstance(payload, dict):
        raise RuntimeError("youtube metadata unavailable")
    return payload


def _extract_transcript(metadata: dict) -> str:
    subtitles = metadata.get("subtitles") or {}
    automatic = metadata.get("automatic_captions") or {}

    for source_name, tracks in (("manual", subtitles), ("automatic", automatic)):
        selected = _select_track(tracks)
        if selected is None:
            continue
        lang, entries = selected
        text = _download_transcript(entries)
        if text:
            return text
    return ""


def _select_track(tracks: dict) -> tuple[str, list[dict]] | None:
    if not isinstance(tracks, dict):
        return None
    normalized: list[tuple[int, str, list[dict]]] = []
    for lang, entries in tracks.items():
        if not isinstance(entries, list) or not entries:
            continue
        rank = _language_rank(str(lang))
        if rank is None:
            continue
        normalized.append((rank, str(lang), entries))
    if not normalized:
        return None
    normalized.sort(key=lambda row: (row[0], row[1]))
    _, lang, entries = normalized[0]
    return lang, entries


def _language_rank(lang: str) -> int | None:
    normalized = (lang or "").strip().lower()
    if not normalized:
        return None
    for index, prefix in enumerate(_LANGUAGE_PREFERENCE):
        if normalized == prefix or normalized.startswith(prefix + "-"):
            return index
    return None


def _download_transcript(entries: list[dict]) -> str:
    preferred = sorted(entries, key=_format_rank)
    for entry in preferred:
        transcript_url = str((entry or {}).get("url") or "").strip()
        if not transcript_url:
            continue
        ext = str((entry or {}).get("ext") or "").strip().lower()
        resp = httpx.get(transcript_url, timeout=30.0, follow_redirects=True)
        resp.raise_for_status()
        body = resp.text or ""
        text = _parse_transcript_text(ext, body)
        if text:
            return text
    return ""


def _format_rank(entry: dict) -> tuple[int, str]:
    ext = str((entry or {}).get("ext") or "").strip().lower()
    try:
        return (_FORMAT_PREFERENCE.index(ext), ext)
    except ValueError:
        return (len(_FORMAT_PREFERENCE), ext)


def _parse_transcript_text(ext: str, body: str) -> str:
    parser = {
        "json3": _parse_json3_transcript,
        "vtt": _parse_vtt_transcript,
    }.get((ext or "").strip().lower())
    if parser is None:
        return ""
    return parser(body)


def _parse_json3_transcript(body: str) -> str:
    try:
        payload = json.loads(body or "{}")
    except Exception:
        return ""
    events = payload.get("events") or []
    lines: list[str] = []
    for event in events:
        segs = (event or {}).get("segs") or []
        text = "".join(str((seg or {}).get("utf8") or "") for seg in segs).strip()
        text = re.sub(r"\s+", " ", text)
        if text:
            lines.append(text)
    return "\n".join(lines).strip()


def _parse_vtt_transcript(body: str) -> str:
    lines: list[str] = []
    for raw_line in (body or "").splitlines():
        line = raw_line.strip()
        if not line:
            continue
        if line == "WEBVTT":
            continue
        if "-->" in line:
            continue
        if line.isdigit():
            continue
        if line.startswith(("NOTE", "STYLE", "REGION")):
            continue
        cleaned = re.sub(r"<[^>]+>", "", line)
        cleaned = re.sub(r"\s+", " ", cleaned).strip()
        if cleaned:
            lines.append(cleaned)
    return "\n".join(lines).strip()


def _normalize_upload_date(raw: str) -> str | None:
    if re.fullmatch(r"\d{8}", raw or ""):
        return f"{raw[0:4]}-{raw[4:6]}-{raw[6:8]}"
    return None
