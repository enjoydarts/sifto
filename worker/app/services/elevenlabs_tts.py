import json
from io import BytesIO

import httpx
from mutagen.mp3 import MP3

_ELEVENLABS_CONTENT_TYPE = "audio/mpeg"
_ELEVENLABS_SUFFIX = ".mp3"


def _estimate_mp3_duration_sec(audio_bytes: bytes) -> int:
    try:
        info = MP3(BytesIO(audio_bytes)).info
        length = float(getattr(info, "length", 0.0) or 0.0)
        if length > 0:
            return max(1, int(round(length)))
    except Exception:
        pass
    return 1


def _raise_http_error(prefix: str, response: httpx.Response, exc: httpx.HTTPStatusError) -> None:
    detail = response.text.strip()
    if detail:
        try:
            detail = json.dumps(response.json(), ensure_ascii=True)
        except ValueError:
            detail = detail[:1000]
        raise RuntimeError(f"{prefix}: status={response.status_code} body={detail}") from exc
    raise RuntimeError(f"{prefix}: status={response.status_code}") from exc


def synthesize_elevenlabs_tts(
    *,
    endpoint: str,
    api_key: str,
    model: str,
    voice_id: str,
    text: str,
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_endpoint = (endpoint or "").strip().rstrip("/")
    normalized_api_key = (api_key or "").strip()
    normalized_model = (model or "").strip()
    normalized_voice_id = (voice_id or "").strip()
    if not normalized_endpoint:
        raise RuntimeError("elevenlabs endpoint is not configured")
    if not normalized_api_key:
        raise RuntimeError("elevenlabs api key is required")
    if not normalized_model:
        raise RuntimeError("elevenlabs tts model is required")
    if not normalized_voice_id:
        raise RuntimeError("elevenlabs voice id is required")

    response = httpx.post(
        f"{normalized_endpoint}/v1/text-to-speech/{normalized_voice_id}",
        headers={
            "xi-api-key": normalized_api_key,
            "Content-Type": "application/json",
            "Accept": "audio/mpeg",
        },
        json={
            "text": text,
            "model_id": normalized_model,
            "output_format": "mp3_44100_128",
        },
        timeout=timeout_sec,
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        _raise_http_error("elevenlabs tts request failed", response, exc)
    audio_bytes = response.content
    return audio_bytes, _ELEVENLABS_CONTENT_TYPE, _ELEVENLABS_SUFFIX, _estimate_mp3_duration_sec(audio_bytes)


def synthesize_elevenlabs_dialogue_tts(
    *,
    endpoint: str,
    api_key: str,
    model: str,
    turns: list[dict[str, str]],
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_endpoint = (endpoint or "").strip().rstrip("/")
    normalized_api_key = (api_key or "").strip()
    normalized_model = (model or "").strip()
    if not normalized_endpoint:
        raise RuntimeError("elevenlabs endpoint is not configured")
    if not normalized_api_key:
        raise RuntimeError("elevenlabs api key is required")
    if not normalized_model:
        raise RuntimeError("elevenlabs tts model is required")
    if normalized_model != "eleven_v3":
        raise RuntimeError("elevenlabs dialogue tts requires eleven_v3 model")
    inputs: list[dict[str, str]] = []
    for turn in turns or []:
        text = str((turn or {}).get("text") or "").strip()
        voice_id = str((turn or {}).get("voice_id") or "").strip()
        if not text or not voice_id:
            continue
        inputs.append({"text": text, "voice_id": voice_id})
    if not inputs:
        raise RuntimeError("elevenlabs dialogue inputs are empty")

    response = httpx.post(
        f"{normalized_endpoint}/v1/text-to-dialogue",
        headers={
            "xi-api-key": normalized_api_key,
            "Content-Type": "application/json",
            "Accept": "audio/mpeg",
        },
        json={
            "inputs": inputs,
            "model_id": normalized_model,
            "output_format": "mp3_44100_128",
        },
        timeout=timeout_sec,
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        _raise_http_error("elevenlabs dialogue request failed", response, exc)
    audio_bytes = response.content
    return audio_bytes, _ELEVENLABS_CONTENT_TYPE, _ELEVENLABS_SUFFIX, _estimate_mp3_duration_sec(audio_bytes)
