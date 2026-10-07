from __future__ import annotations

import json
import logging
import os
import re
import subprocess
import xml.etree.ElementTree as ET
from urllib.parse import parse_qs, urlparse

from app.services.bounded_download import download_bytes
from app.services.url_security import ensure_response_size, validate_public_http_url

_log = logging.getLogger(__name__)

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
_YTDLP_ERROR_LIMIT = 500
_YTDLP_DEBUG_SNIPPET_LIMIT = 800


class YouTubeTranscriptUnavailableError(RuntimeError):
    def __init__(
        self,
        *,
        title: str,
        published_at: str | None,
        image_url: str | None,
        diagnostics: str,
    ):
        message = "youtube transcript unavailable"
        if diagnostics:
            message = f"{message}: {diagnostics}"
        super().__init__(message)
        self.title = title
        self.published_at = published_at
        self.image_url = image_url


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
    url = validate_public_http_url(url)
    extractor_args = (os.getenv("YTDLP_EXTRACTOR_ARGS") or "").strip()
    pot_provider_present = bool((os.getenv("YTDLP_POT_PROVIDER_BASE_URL") or "").strip())
    pot_provider_args = _build_pot_provider_extractor_args()
    metadata = _load_video_metadata(url)
    title = str(metadata.get("title") or "").strip()
    if not title:
        raise RuntimeError("youtube metadata unavailable")

    published_at = _normalize_upload_date(str(metadata.get("upload_date") or "").strip())
    image_url = str(metadata.get("thumbnail") or "").strip() or None
    transcript = _extract_transcript(metadata)
    if not transcript:
        raise YouTubeTranscriptUnavailableError(
            title=title,
            published_at=published_at,
            image_url=image_url,
            diagnostics="no supported transcript",
        )

    return {
        "title": title,
        "content": transcript,
        "published_at": published_at,
        "image_url": image_url,
    }


def _load_video_metadata(url: str) -> dict:
    cmd = _build_ytdlp_metadata_command(verbose=False)
    extractor_args = (os.getenv("YTDLP_EXTRACTOR_ARGS") or "").strip()
    pot_provider_args = _build_pot_provider_extractor_args()
    _log.info(
        "youtube metadata fetch url=%s extractor_args_present=%s pot_provider_present=%s",
        url,
        bool(extractor_args),
        bool(pot_provider_args),
    )
    if extractor_args:
        cmd.extend(["--extractor-args", extractor_args])
    if pot_provider_args:
        cmd.extend(["--extractor-args", pot_provider_args])
    cmd.append(url)
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True, timeout=35)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError("yt-dlp metadata fetch failed") from exc
    payload = json.loads(proc.stdout or "{}")
    if not isinstance(payload, dict):
        raise RuntimeError("youtube metadata unavailable")
    return payload


def _build_pot_provider_extractor_args() -> str:
    base_url = (os.getenv("YTDLP_POT_PROVIDER_BASE_URL") or "").strip()
    if not base_url:
        return ""
    parts = [f"base_url={base_url}"]
    if _env_truthy("YTDLP_POT_PROVIDER_DISABLE_INNERTUBE"):
        parts.append("disable_innertube=1")
    return "youtubepot-bgutilhttp:" + ";".join(parts)


def _build_ytdlp_metadata_command(*, verbose: bool) -> list[str]:
    # Never inherit a host config containing shared account credentials.
    cmd = ["yt-dlp", "--ignore-config"]
    if verbose:
        cmd.append("-v")
    cmd.extend(["--dump-single-json", "--no-warnings", "--skip-download", "--ignore-no-formats-error", "--no-playlist", "--socket-timeout", "10", "--retries", "1", "--extractor-retries", "1"])
    return cmd


def _collect_ytdlp_debug_details(
    url: str,
    extractor_args: str,
    pot_provider_args: str,
) -> str:
    debug_cmd = _build_ytdlp_metadata_command(verbose=True)
    if extractor_args:
        debug_cmd.extend(["--extractor-args", extractor_args])
    if pot_provider_args:
        debug_cmd.extend(["--extractor-args", pot_provider_args])
    debug_cmd.append(url)
    try:
        proc = subprocess.run(debug_cmd, capture_output=True, text=True, check=False, timeout=10)
    except Exception as exc:
        return f"verbose_run_failed={type(exc).__name__}"
    merged = "\n".join(part for part in [proc.stderr or "", proc.stdout or ""] if part).strip()
    if not merged:
        return ""
    focus_lines: list[str] = []
    for raw_line in merged.splitlines():
        line = raw_line.strip()
        lower = line.lower()
        if "po token providers" in lower or "youtubepot" in lower or "bgutil" in lower or "plugin" in lower:
            focus_lines.append(line)
    if not focus_lines:
        focus_lines = [line.strip() for line in merged.splitlines()[:8] if line.strip()]
    snippet = " | ".join(focus_lines)
    snippet = re.sub(r"\s+", " ", snippet).strip()
    if len(snippet) > _YTDLP_DEBUG_SNIPPET_LIMIT:
        snippet = snippet[:_YTDLP_DEBUG_SNIPPET_LIMIT] + "..."
    return snippet


def _env_truthy(name: str) -> bool:
    value = (os.getenv(name) or "").strip().lower()
    return value in {"1", "true", "yes", "on"}


def _truncate_error_detail(detail: str) -> str:
    value = re.sub(r"\s+", " ", (detail or "").strip())
    if not value:
        return "unknown error"
    if len(value) > _YTDLP_ERROR_LIMIT:
        return value[:_YTDLP_ERROR_LIMIT] + "..."
    return value


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


def _describe_available_transcripts(metadata: dict) -> str:
    subtitles = metadata.get("subtitles") or {}
    automatic = metadata.get("automatic_captions") or {}
    return " ".join(
        [
            _describe_track_set("manual_langs", subtitles),
            _describe_track_exts("manual_exts", subtitles),
            _describe_track_set("auto_langs", automatic),
            _describe_track_exts("auto_exts", automatic),
        ]
    ).strip()


def _describe_track_set(label: str, tracks: dict) -> str:
    if not isinstance(tracks, dict) or not tracks:
        return f"{label}=[]"
    langs = sorted(str(lang) for lang, entries in tracks.items() if isinstance(entries, list) and entries)
    return f"{label}={langs}"


def _describe_track_exts(label: str, tracks: dict) -> str:
    if not isinstance(tracks, dict) or not tracks:
        return f"{label}=[]"
    exts: set[str] = set()
    for entries in tracks.values():
        if not isinstance(entries, list):
            continue
        for entry in entries:
            ext = str((entry or {}).get("ext") or "").strip().lower()
            if ext:
                exts.add(ext)
    return f"{label}={sorted(exts)}"


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
    preferred = sorted(entries, key=_format_rank)[:3]
    for entry in preferred:
        transcript_url = str((entry or {}).get("url") or "").strip()
        if not transcript_url:
            continue
        ext = str((entry or {}).get("ext") or "").strip().lower()
        content, _ = download_bytes(transcript_url, 5 * 1024 * 1024, timeout_sec=5.0)
        body = content.decode("utf-8", errors="replace")
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
        "srv3": _parse_xml_transcript,
        "srv2": _parse_xml_transcript,
        "srv1": _parse_xml_transcript,
        "ttml": _parse_xml_transcript,
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


def _parse_xml_transcript(body: str) -> str:
    try:
        root = ET.fromstring(body or "")
    except ET.ParseError:
        return ""

    paragraph_nodes = [node for node in root.iter() if node.tag.rsplit("}", 1)[-1].lower() == "p"]
    candidate_nodes = paragraph_nodes if paragraph_nodes else [node for node in root.iter() if node.tag.rsplit("}", 1)[-1].lower() in {"s", "span"}]

    lines: list[str] = []
    for node in candidate_nodes:
        text = "".join(node.itertext())
        text = re.sub(r"\s+", " ", text).strip()
        if text:
            lines.append(text)
    return "\n".join(lines).strip()


def _normalize_upload_date(raw: str) -> str | None:
    if re.fullmatch(r"\d{8}", raw or ""):
        return f"{raw[0:4]}-{raw[4:6]}-{raw[6:8]}"
    return None
