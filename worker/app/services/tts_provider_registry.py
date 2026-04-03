from app.services.openai_tts import synthesize_openai_tts
from app.services.xai_tts import synthesize_xai_tts


def synthesize_catalog_tts(
    provider: str,
    *,
    endpoint: str,
    api_key: str,
    voice_id: str,
    tts_model: str,
    text: str,
    speech_rate: float,
    timeout_sec: float,
) -> tuple[bytes, str, str, int]:
    normalized_provider = (provider or "").strip().lower()
    if normalized_provider == "xai":
        return synthesize_xai_tts(
            endpoint=endpoint,
            api_key=api_key,
            voice_id=voice_id,
            text=text,
            speech_rate=speech_rate,
            timeout_sec=timeout_sec,
        )
    if normalized_provider == "openai":
        return synthesize_openai_tts(
            endpoint=endpoint,
            api_key=api_key,
            model=tts_model,
            voice_id=voice_id,
            text=text,
            speech_rate=speech_rate,
            timeout_sec=timeout_sec,
        )
    raise RuntimeError(f"unsupported catalog tts provider: {provider}")
