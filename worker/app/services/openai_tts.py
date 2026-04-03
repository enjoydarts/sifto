import json
import math

import httpx


def estimate_audio_duration_sec(text: str, speech_rate: float) -> int:
    normalized_rate = speech_rate if speech_rate > 0 else 1.0
    return max(1, int(math.ceil(max(len(text or ""), 12) / 14 / normalized_rate)))


def synthesize_openai_tts(
    *,
    endpoint: str,
    api_key: str,
    model: str,
    voice_id: str,
    text: str,
    speech_rate: float,
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_endpoint = (endpoint or "").strip().rstrip("/")
    normalized_api_key = (api_key or "").strip()
    normalized_model = (model or "").strip()
    normalized_voice_id = (voice_id or "").strip()
    if not normalized_endpoint:
        raise RuntimeError("openai tts endpoint is not configured")
    if not normalized_api_key:
        raise RuntimeError("openai api key is required")
    if not normalized_model:
        raise RuntimeError("openai tts model is required")
    if not normalized_voice_id:
        raise RuntimeError("openai voice id is required")

    response = httpx.post(
        f"{normalized_endpoint}/v1/audio/speech",
        headers={"Authorization": f"Bearer {normalized_api_key}"},
        json={
            "model": normalized_model,
            "voice": normalized_voice_id,
            "input": text,
            "language": "ja",
            "response_format": "mp3",
        },
        timeout=timeout_sec,
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        detail = ""
        body = response.text.strip()
        if body:
            try:
                parsed = response.json()
                detail = json.dumps(parsed, ensure_ascii=True)
            except ValueError:
                detail = body[:1000]
        if detail:
            raise RuntimeError(f"openai tts request failed: status={response.status_code} body={detail}") from exc
        raise
    return response.content, "audio/mpeg", ".mp3", estimate_audio_duration_sec(text, speech_rate)
