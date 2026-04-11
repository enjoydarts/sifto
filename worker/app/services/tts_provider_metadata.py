import os
from dataclasses import dataclass


def _env_float(name: str, default: float) -> float:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


@dataclass(frozen=True)
class SingleSpeakerTTSProviderRuntimeMetadata:
    endpoint: str
    api_key: str
    timeout_sec: float


def load_single_speaker_tts_provider_runtime_metadata(provider: str) -> SingleSpeakerTTSProviderRuntimeMetadata:
    normalized_provider = (provider or "").strip().lower()
    if normalized_provider == "xai":
        return SingleSpeakerTTSProviderRuntimeMetadata(
            endpoint=(os.getenv("XAI_TTS_ENDPOINT", "https://api.x.ai").strip() or "https://api.x.ai").rstrip("/"),
            api_key=os.getenv("XAI_API_KEY", "").strip(),
            timeout_sec=max(_env_float("XAI_TTS_TIMEOUT_SEC", 300.0), 1.0),
        )
    if normalized_provider == "gemini_tts":
        return SingleSpeakerTTSProviderRuntimeMetadata(
            endpoint=(os.getenv("GEMINI_TTS_ENDPOINT", "https://generativelanguage.googleapis.com").strip() or "https://generativelanguage.googleapis.com").rstrip("/"),
            api_key=os.getenv("GEMINI_API_KEY", "").strip(),
            timeout_sec=max(_env_float("GEMINI_TTS_TIMEOUT_SEC", 300.0), 1.0),
        )
    if normalized_provider == "fish":
        return SingleSpeakerTTSProviderRuntimeMetadata(
            endpoint="",
            api_key=os.getenv("FISH_API_KEY", "").strip(),
            timeout_sec=max(_env_float("FISH_TTS_TIMEOUT_SEC", 300.0), 1.0),
        )
    if normalized_provider == "elevenlabs":
        return SingleSpeakerTTSProviderRuntimeMetadata(
            endpoint=(os.getenv("ELEVENLABS_TTS_ENDPOINT", "https://api.elevenlabs.io").strip() or "https://api.elevenlabs.io").rstrip("/"),
            api_key=os.getenv("ELEVENLABS_API_KEY", "").strip(),
            timeout_sec=max(_env_float("ELEVENLABS_TTS_TIMEOUT_SEC", 300.0), 1.0),
        )
    if normalized_provider == "openai":
        return SingleSpeakerTTSProviderRuntimeMetadata(
            endpoint=(os.getenv("OPENAI_TTS_ENDPOINT", "https://api.openai.com").strip() or "https://api.openai.com").rstrip("/"),
            api_key=os.getenv("OPENAI_API_KEY", "").strip(),
            timeout_sec=max(_env_float("OPENAI_TTS_TIMEOUT_SEC", 300.0), 1.0),
        )
    raise RuntimeError(f"unsupported single-speaker tts provider metadata: {provider}")
