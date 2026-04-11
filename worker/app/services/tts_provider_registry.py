from app.services.azure_speech_tts import synthesize_azure_speech_tts
from app.services.elevenlabs_tts import synthesize_elevenlabs_tts
from app.services.fish_tts import synthesize_fish_tts
from app.services.gemini_tts import synthesize_gemini_tts
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


def synthesize_single_speaker_tts(
    provider: str,
    *,
    endpoint: str,
    api_key: str,
    region: str,
    voice_id: str,
    tts_model: str,
    text: str,
    speech_rate: float,
    timeout_sec: float,
    persona: str = "",
    volume_gain: float = 0.0,
    line_break_silence_seconds: float = 0.4,
    pitch: float = 0.0,
) -> tuple[bytes, str, str, int]:
    normalized_provider = (provider or "").strip().lower()
    if normalized_provider == "azure_speech":
        return synthesize_azure_speech_tts(
            region=region,
            api_key=api_key,
            voice_name=voice_id,
            text=text,
            speech_rate=speech_rate,
            line_break_silence_seconds=line_break_silence_seconds,
            pitch=pitch,
            volume_gain=volume_gain,
            timeout_sec=timeout_sec,
        )
    if normalized_provider in {"xai", "openai"}:
        return synthesize_catalog_tts(
            normalized_provider,
            endpoint=endpoint,
            api_key=api_key,
            voice_id=voice_id,
            tts_model=tts_model,
            text=text,
            speech_rate=speech_rate,
            timeout_sec=timeout_sec,
        )
    if normalized_provider == "gemini_tts":
        normalized_tts_model = (tts_model or "").strip()
        if not normalized_tts_model:
            raise RuntimeError("gemini tts model is required")
        return synthesize_gemini_tts(
            model=normalized_tts_model,
            voice_name=voice_id,
            persona=persona,
            text=text,
            speech_rate=speech_rate,
            api_key=(api_key or "").strip() or None,
        )
    if normalized_provider == "fish":
        normalized_tts_model = (tts_model or "").strip()
        if not normalized_tts_model:
            raise RuntimeError("fish tts model is required")
        return synthesize_fish_tts(
            model=normalized_tts_model,
            voice_name=voice_id,
            text=text,
            speech_rate=speech_rate,
            volume_gain=volume_gain,
            api_key=api_key,
            timeout_sec=timeout_sec,
        )
    if normalized_provider == "elevenlabs":
        return synthesize_elevenlabs_tts(
            endpoint=endpoint,
            api_key=api_key,
            model=tts_model,
            voice_id=voice_id,
            text=text,
            timeout_sec=timeout_sec,
        )
        raise RuntimeError(f"unsupported single-speaker tts provider: {provider}")
