import math

import httpx


def estimate_audio_duration_sec(text: str, speech_rate: float) -> int:
    normalized_rate = speech_rate if speech_rate > 0 else 1.0
    return max(1, int(math.ceil(max(len(text or ""), 12) / 14 / normalized_rate)))


def synthesize_xai_tts(
    *,
    endpoint: str,
    api_key: str,
    voice_id: str,
    text: str,
    speech_rate: float,
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_endpoint = (endpoint or "").strip().rstrip("/")
    normalized_api_key = (api_key or "").strip()
    if not normalized_endpoint:
        raise RuntimeError("xai tts endpoint is not configured")
    if not normalized_api_key:
        raise RuntimeError("xai api key is required")

    response = httpx.post(
        f"{normalized_endpoint}/v1/tts",
        headers={"Authorization": f"Bearer {normalized_api_key}"},
        json={"input": text, "voice_id": voice_id, "format": "mp3"},
        timeout=timeout_sec,
    )
    response.raise_for_status()
    return response.content, "audio/mpeg", ".mp3", estimate_audio_duration_sec(text, speech_rate)
