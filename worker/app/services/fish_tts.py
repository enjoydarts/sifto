import json
import math
import os
from io import BytesIO

import httpx
from mutagen.mp3 import MP3

_FISH_TTS_CONTENT_TYPE = "audio/mpeg"
_FISH_TTS_SUFFIX = ".mp3"
_FISH_TTS_SAMPLE_RATE = 44100
_FISH_TTS_MP3_BITRATE = 192
_FISH_TTS_CHUNK_LENGTH = 300


def _resolve_fish_tts_endpoint() -> str:
    return (os.getenv("FISH_TTS_ENDPOINT", "https://api.fish.audio").strip() or "https://api.fish.audio").rstrip("/")


def _resolve_fish_api_key(api_key_override: str | None = None) -> str:
    normalized_override = (api_key_override or "").strip()
    if normalized_override:
        return normalized_override
    normalized_env = os.getenv("FISH_API_KEY", "").strip()
    if normalized_env:
        return normalized_env
    raise RuntimeError("fish api key is required")


def _normalize_speech_rate(speech_rate: float) -> float:
    try:
        normalized = float(speech_rate)
    except (TypeError, ValueError):
        return 1.0
    return normalized if normalized > 0 else 1.0


def _estimate_mp3_duration_sec(audio_bytes: bytes) -> int:
    try:
        info = MP3(BytesIO(audio_bytes)).info
        length = float(getattr(info, "length", 0.0) or 0.0)
        if length > 0:
            return max(1, int(math.ceil(length)))
    except Exception:
        pass
    return 1


def _build_fish_headers(api_key: str, model: str) -> dict[str, str]:
    normalized_api_key = (api_key or "").strip()
    normalized_model = (model or "").strip()
    if not normalized_api_key:
        raise RuntimeError("fish api key is required")
    if not normalized_model:
        raise RuntimeError("fish tts model is required")
    return {
        "Authorization": f"Bearer {normalized_api_key}",
        "Content-Type": "application/json",
        "model": normalized_model,
    }


def _build_fish_payload(
    *,
    text: str,
    reference_id: str | list[str],
    speech_rate: float,
    volume_gain: float,
) -> dict:
    normalized_text = (text or "").strip()
    if not normalized_text:
        raise RuntimeError("fish tts text is empty")
    normalized_rate = _normalize_speech_rate(speech_rate)
    try:
        normalized_volume = float(volume_gain)
    except (TypeError, ValueError):
        normalized_volume = 0.0
    payload = {
        "text": normalized_text,
        "reference_id": reference_id,
        "prosody": {
            "speed": normalized_rate,
            "volume": normalized_volume,
        },
        "chunk_length": _FISH_TTS_CHUNK_LENGTH,
        "normalize": True,
        "format": "mp3",
        "sample_rate": _FISH_TTS_SAMPLE_RATE,
        "mp3_bitrate": _FISH_TTS_MP3_BITRATE,
        "latency": "balanced",
        "temperature": 0.7,
        "top_p": 0.7,
    }
    return payload


def _synthesize_fish_tts(
    *,
    model: str,
    reference_id: str | list[str],
    text: str,
    speech_rate: float,
    volume_gain: float,
    api_key: str | None = None,
    timeout_sec: float | None = None,
) -> tuple[bytes, str, str, int]:
    normalized_model = (model or "").strip()
    if not normalized_model:
        raise RuntimeError("fish tts model is required")
    normalized_reference_id = reference_id
    if isinstance(normalized_reference_id, list):
        cleaned_reference_ids = [str(item or "").strip() for item in normalized_reference_id if str(item or "").strip()]
        if not cleaned_reference_ids:
            raise RuntimeError("fish reference_id is required")
        normalized_reference_id = cleaned_reference_ids
    else:
        normalized_reference_id = (str(normalized_reference_id or "").strip() or "")
        if not normalized_reference_id:
            raise RuntimeError("fish reference_id is required")

    response = httpx.post(
        f"{_resolve_fish_tts_endpoint()}/v1/tts",
        headers=_build_fish_headers(_resolve_fish_api_key(api_key), normalized_model),
        json=_build_fish_payload(
            text=text,
            reference_id=normalized_reference_id,
            speech_rate=speech_rate,
            volume_gain=volume_gain,
        ),
        timeout=max(float(timeout_sec or os.getenv("FISH_TTS_TIMEOUT_SEC", "300") or "300"), 1.0),
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        detail = ""
        body = response.text.strip()
        if body:
            try:
                detail = json.dumps(response.json(), ensure_ascii=True)
            except ValueError:
                detail = body[:1000]
        if detail:
            raise RuntimeError(f"fish tts request failed: status={response.status_code} body={detail}") from exc
        raise
    audio_bytes = response.content
    if not audio_bytes:
        raise RuntimeError("fish tts response did not include audio data")
    duration_sec = _estimate_mp3_duration_sec(audio_bytes)
    return audio_bytes, _FISH_TTS_CONTENT_TYPE, _FISH_TTS_SUFFIX, duration_sec


def synthesize_fish_tts(
    *,
    model: str,
    voice_name: str,
    text: str,
    speech_rate: float,
    volume_gain: float = 0.0,
    api_key: str | None = None,
    timeout_sec: float | None = None,
) -> tuple[bytes, str, str, int]:
    normalized_voice_name = (voice_name or "").strip()
    if not normalized_voice_name:
        raise RuntimeError("fish voice name is required")
    normalized_model = (model or "").strip()
    if normalized_model not in {"s1", "s2-pro"}:
        raise RuntimeError("fish tts model must be s1 or s2-pro")
    return _synthesize_fish_tts(
        model=normalized_model,
        reference_id=normalized_voice_name,
        text=text,
        speech_rate=speech_rate,
        volume_gain=volume_gain,
        api_key=api_key,
        timeout_sec=timeout_sec,
    )


def build_fish_duo_text(turns: list[dict[str, str]]) -> str:
    lines: list[str] = []
    for turn in turns or []:
        speaker = str((turn or {}).get("speaker") or "").strip().lower()
        text = str((turn or {}).get("text") or "").strip()
        if speaker not in {"host", "partner"} or not text:
            continue
        speaker_index = "0" if speaker == "host" else "1"
        lines.append(f"<|speaker:{speaker_index}|>{text}")
    return "".join(lines)


def synthesize_fish_multi_speaker_tts(
    *,
    model: str,
    host_voice_name: str,
    partner_voice_name: str,
    turns: list[dict[str, str]],
    text: str | None = None,
    api_key: str | None = None,
    timeout_sec: float | None = None,
) -> tuple[bytes, str, str, int]:
    normalized_model = (model or "").strip()
    if normalized_model != "s2-pro":
        raise RuntimeError("fish duo requires s2-pro")
    normalized_host_voice_name = (host_voice_name or "").strip()
    if not normalized_host_voice_name:
        raise RuntimeError("fish host voice name is required")
    normalized_partner_voice_name = (partner_voice_name or "").strip()
    if not normalized_partner_voice_name:
        raise RuntimeError("fish partner voice name is required")
    if normalized_host_voice_name == normalized_partner_voice_name:
        raise RuntimeError("fish duo requires distinct host and partner voices")
    dialogue_text = str(text or "").strip() or build_fish_duo_text(turns)
    if not dialogue_text:
        raise RuntimeError("fish duo turns are empty")
    return _synthesize_fish_tts(
        model=normalized_model,
        reference_id=[normalized_host_voice_name, normalized_partner_voice_name],
        text=dialogue_text,
        speech_rate=1.0,
        volume_gain=0.0,
        api_key=api_key,
        timeout_sec=timeout_sec,
    )
