import math
import re
import xml.etree.ElementTree as ET
from xml.sax.saxutils import escape

import httpx

_AZURE_SPEECH_CONTENT_TYPE = "audio/mpeg"
_AZURE_SPEECH_SUFFIX = ".mp3"
_AZURE_SPEECH_OUTPUT_FORMAT = "audio-24khz-96kbitrate-mono-mp3"
_AZURE_SPEECH_SSML_NS = "http://www.w3.org/2001/10/synthesis"
_AZURE_SPEECH_MSTTS_NS = "https://www.w3.org/2001/mstts"
_AZURE_SPEECH_XML_NS = "http://www.w3.org/XML/1998/namespace"

_XML_10_INVALID_CHAR_RE = re.compile(
    r"[^\u0009\u000A\u000D\u0020-\uD7FF\uE000-\uFFFD]"
)


def _normalize_region(region: str) -> str:
    value = (region or "").strip()
    if not value:
        raise RuntimeError("azure speech region is required")
    return value


def _normalize_api_key(api_key: str) -> str:
    value = (api_key or "").strip()
    if not value:
        raise RuntimeError("azure speech api key is required")
    return value


def _normalize_voice_name(voice_name: str) -> str:
    value = (voice_name or "").strip()
    if not value:
        raise RuntimeError("azure speech voice name is required")
    return value


def _normalize_timeout(timeout_sec: float) -> float:
    return max(float(timeout_sec or 0.0), 1.0)


def _strip_invalid_xml_chars(value: str) -> str:
    return _XML_10_INVALID_CHAR_RE.sub("", value or "")


def _prosody_rate_attr(speech_rate: float) -> str:
    rate = speech_rate if speech_rate > 0 else 1.0
    if abs(rate - 1.0) < 0.001:
        return ""
    return f"{rate:.2f}".rstrip("0").rstrip(".")


def _prosody_pitch_attr(pitch: float) -> str:
    percent = int(round(float(pitch or 0.0) * 10))
    if percent == 0:
        return ""
    return f"{percent:+d}%"


def _prosody_volume_attr(volume_gain: float) -> str:
    gain = float(volume_gain or 0.0)
    if abs(gain) < 0.001:
        return ""
    return f"{gain:+.1f}"


def _prosody_open_tag(speech_rate: float, pitch: float, volume_gain: float) -> str:
    attrs: list[str] = []
    rate_attr = _prosody_rate_attr(speech_rate)
    if rate_attr:
        attrs.append(f'rate="{rate_attr}"')
    pitch_attr = _prosody_pitch_attr(pitch)
    if pitch_attr:
        attrs.append(f'pitch="{pitch_attr}"')
    volume_attr = _prosody_volume_attr(volume_gain)
    if volume_attr:
        attrs.append(f'volume="{volume_attr}"')
    if not attrs:
        return ""
    return f"<prosody {' '.join(attrs)}>"


def _looks_like_ssml(text: str) -> bool:
    return (text or "").lstrip().startswith("<speak")


def _local_name(tag: str) -> str:
    if "}" in tag:
        return tag.split("}", 1)[1]
    return tag


def _namespace(tag: str) -> str:
    if tag.startswith("{") and "}" in tag:
        return tag[1:].split("}", 1)[0]
    return ""


def _validate_azure_speech_ssml(ssml: str, *, allowed_voice_names: set[str]) -> str:
    normalized = _strip_invalid_xml_chars(ssml).strip()
    if not normalized:
        raise RuntimeError("azure speech ssml is empty")
    try:
        root = ET.fromstring(normalized)
    except ET.ParseError as exc:
        raise RuntimeError(f"invalid azure speech ssml: {exc}") from exc
    if _local_name(root.tag) != "speak" or _namespace(root.tag) != _AZURE_SPEECH_SSML_NS:
        raise RuntimeError("azure speech ssml root must be <speak> in synthesis namespace")
    root_lang = root.attrib.get(f"{{{_AZURE_SPEECH_XML_NS}}}lang", "").strip()
    if root_lang and root_lang != "ja-JP":
        raise RuntimeError("azure speech ssml root xml:lang must be ja-JP")

    allowed_tags = {
        (_AZURE_SPEECH_SSML_NS, "speak"),
        (_AZURE_SPEECH_SSML_NS, "voice"),
        (_AZURE_SPEECH_SSML_NS, "prosody"),
        (_AZURE_SPEECH_SSML_NS, "break"),
        (_AZURE_SPEECH_SSML_NS, "emphasis"),
        (_AZURE_SPEECH_MSTTS_NS, "express-as"),
    }
    allowed_attrs = {
        "speak": {f"{{{_AZURE_SPEECH_XML_NS}}}lang", "version"},
        "voice": {f"{{{_AZURE_SPEECH_XML_NS}}}lang", "name"},
        "prosody": {"rate", "pitch", "volume"},
        "break": {"time", "strength"},
        "emphasis": {"level"},
        "express-as": {"style", "styledegree", "role"},
    }

    seen_voice_names: set[str] = set()
    for elem in root.iter():
        key = (_namespace(elem.tag), _local_name(elem.tag))
        if key not in allowed_tags:
            raise RuntimeError(f"unsupported azure speech ssml tag: {_local_name(elem.tag)}")
        tag_name = _local_name(elem.tag)
        for attr in elem.attrib:
            if attr not in allowed_attrs.get(tag_name, set()):
                raise RuntimeError(f"unsupported azure speech ssml attribute: {tag_name}.{_local_name(attr)}")
        if tag_name == "voice":
            voice_name = elem.attrib.get("name", "").strip()
            if voice_name == "":
                raise RuntimeError("azure speech ssml voice.name is required")
            if voice_name not in allowed_voice_names:
                raise RuntimeError(f"azure speech ssml voice.name is not allowed: {voice_name}")
            voice_lang = elem.attrib.get(f"{{{_AZURE_SPEECH_XML_NS}}}lang", "").strip()
            if voice_lang and voice_lang != "ja-JP":
                raise RuntimeError("azure speech ssml voice xml:lang must be ja-JP")
            seen_voice_names.add(voice_name)
    if not seen_voice_names:
        raise RuntimeError("azure speech ssml must include at least one voice")
    return normalized


def _build_text_with_breaks(text: str, line_break_silence_seconds: float) -> str:
    normalized = _strip_invalid_xml_chars(text).replace("\r\n", "\n").replace("\r", "\n")
    normalized = normalized.strip()
    if not normalized:
        raise RuntimeError("azure speech text is required")
    break_ms = max(int(round(max(float(line_break_silence_seconds or 0.0), 0.0) * 1000)), 0)
    paragraph_break = f'<break time="{break_ms}ms"/>' if break_ms > 0 else "<break strength=\"medium\"/>"
    line_break = f'<break time="{max(break_ms // 2, 150)}ms"/>' if break_ms > 0 else "<break strength=\"weak\"/>"
    paragraphs = [segment.strip() for segment in re.split(r"\n{2,}", normalized) if segment.strip()]
    rendered_paragraphs: list[str] = []
    for paragraph in paragraphs:
        lines = [escape(line.strip()) for line in paragraph.split("\n") if line.strip()]
        rendered_paragraphs.append(line_break.join(lines))
    return paragraph_break.join(rendered_paragraphs)


def _build_single_voice_ssml(
    *,
    voice_name: str,
    text: str,
    speech_rate: float,
    line_break_silence_seconds: float,
    pitch: float,
    volume_gain: float,
) -> str:
    inner = _build_text_with_breaks(text, line_break_silence_seconds)
    prosody_open = _prosody_open_tag(speech_rate, pitch, volume_gain)
    prosody_close = "</prosody>" if prosody_open else ""
    return (
        '<speak version="1.0" xml:lang="ja-JP" xmlns="http://www.w3.org/2001/10/synthesis">'
        f'<voice xml:lang="ja-JP" name="{escape(voice_name)}">'
        f"{prosody_open}"
        f"{inner}"
        f"{prosody_close}"
        "</voice>"
        "</speak>"
    )


def build_azure_speech_duo_ssml(
    *,
    host_voice_name: str,
    partner_voice_name: str,
    turns: list[dict[str, str]],
    speech_rate: float,
    line_break_silence_seconds: float,
    pitch: float,
    volume_gain: float,
) -> str:
    voice_by_speaker = {
        "host": _normalize_voice_name(host_voice_name),
        "partner": _normalize_voice_name(partner_voice_name),
    }
    rendered_turns: list[str] = []
    prosody_open = _prosody_open_tag(speech_rate, pitch, volume_gain)
    prosody_close = "</prosody>" if prosody_open else ""
    for turn in turns or []:
        speaker = str((turn or {}).get("speaker") or "").strip().lower()
        if speaker not in voice_by_speaker:
            speaker = "host"
        text = str((turn or {}).get("text") or "").strip()
        if not text:
            continue
        rendered_turns.append(
            f'<voice xml:lang="ja-JP" name="{escape(voice_by_speaker[speaker])}">'
            f"{prosody_open}"
            f"{_build_text_with_breaks(text, line_break_silence_seconds)}"
            f"{prosody_close}"
            "</voice>"
        )
    if not rendered_turns:
        raise RuntimeError("azure speech duo turns are empty")
    return (
        '<speak version="1.0" xml:lang="ja-JP" xmlns="http://www.w3.org/2001/10/synthesis">'
        + "".join(rendered_turns)
        + "</speak>"
    )


def _post_ssml(*, region: str, api_key: str, ssml: str, timeout_sec: float) -> tuple[bytes, str, str, int]:
    normalized_region = _normalize_region(region)
    normalized_api_key = _normalize_api_key(api_key)
    response = httpx.post(
        f"https://{normalized_region}.tts.speech.microsoft.com/cognitiveservices/v1",
        headers={
            "Ocp-Apim-Subscription-Key": normalized_api_key,
            "Accept": _AZURE_SPEECH_CONTENT_TYPE,
            "Content-Type": "application/ssml+xml",
            "X-Microsoft-OutputFormat": _AZURE_SPEECH_OUTPUT_FORMAT,
            "User-Agent": "sifto-worker",
        },
        content=ssml.encode("utf-8"),
        timeout=_normalize_timeout(timeout_sec),
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        detail = (exc.response.text or "").strip()
        request_id = (response.headers.get("X-RequestId") or response.headers.get("x-requestid") or "").strip()
        if not detail:
            detail = "<empty>"
        if request_id:
            detail = f"{detail} request_id={request_id}"
        raise RuntimeError(f"azure speech request failed: status={response.status_code} body={detail[:1000]}") from exc
    audio_bytes = bytes(response.content)
    return audio_bytes, _AZURE_SPEECH_CONTENT_TYPE, _AZURE_SPEECH_SUFFIX, _estimate_mp3_duration_sec(audio_bytes)


def synthesize_azure_speech_tts(
    *,
    region: str,
    api_key: str,
    voice_name: str,
    text: str,
    speech_rate: float,
    line_break_silence_seconds: float,
    pitch: float,
    volume_gain: float,
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_voice_name = _normalize_voice_name(voice_name)
    if _looks_like_ssml(text):
        ssml = _validate_azure_speech_ssml(text, allowed_voice_names={normalized_voice_name})
    else:
        ssml = _build_single_voice_ssml(
            voice_name=normalized_voice_name,
            text=text,
            speech_rate=speech_rate,
            line_break_silence_seconds=line_break_silence_seconds,
            pitch=pitch,
            volume_gain=volume_gain,
        )
    return _post_ssml(region=region, api_key=api_key, ssml=ssml, timeout_sec=timeout_sec)


def synthesize_azure_speech_duo_tts(
    *,
    region: str,
    api_key: str,
    host_voice_name: str,
    partner_voice_name: str,
    turns: list[dict[str, str]],
    preprocessed_text: str | None,
    speech_rate: float,
    line_break_silence_seconds: float,
    pitch: float,
    volume_gain: float,
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_host_voice_name = _normalize_voice_name(host_voice_name)
    normalized_partner_voice_name = _normalize_voice_name(partner_voice_name)
    if _looks_like_ssml(preprocessed_text or ""):
        ssml = _validate_azure_speech_ssml(
            preprocessed_text or "",
            allowed_voice_names={normalized_host_voice_name, normalized_partner_voice_name},
        )
    else:
        ssml = build_azure_speech_duo_ssml(
            host_voice_name=normalized_host_voice_name,
            partner_voice_name=normalized_partner_voice_name,
            turns=turns,
            speech_rate=speech_rate,
            line_break_silence_seconds=line_break_silence_seconds,
            pitch=pitch,
            volume_gain=volume_gain,
        )
    return _post_ssml(region=region, api_key=api_key, ssml=ssml, timeout_sec=timeout_sec)


def _estimate_mp3_duration_sec(audio_bytes: bytes) -> int:
    size = len(audio_bytes)
    if size <= 0:
        return 1
    bitrate_bytes_per_sec = 96_000 / 8
    return max(1, int(math.ceil(size / bitrate_bytes_per_sec)))
