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
    region: str = ""


# Declarative table for single-speaker TTS providers.
# Adding a covered provider: add entry here + implement synthesize if needed. No if-chain extension.
_SINGLE_SPEAKER_TTS_METADATA = {
    "xai": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint=(os.getenv("XAI_TTS_ENDPOINT", "https://api.x.ai").strip() or "https://api.x.ai").rstrip("/"),
        api_key=os.getenv("XAI_API_KEY", "").strip(),
        timeout_sec=max(_env_float("XAI_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region="",
    ),
    "gemini_tts": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint=(os.getenv("GEMINI_TTS_ENDPOINT", "https://generativelanguage.googleapis.com").strip() or "https://generativelanguage.googleapis.com").rstrip("/"),
        api_key=os.getenv("GEMINI_API_KEY", "").strip(),
        timeout_sec=max(_env_float("GEMINI_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region="",
    ),
    "fish": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint="",
        api_key=os.getenv("FISH_API_KEY", "").strip(),
        timeout_sec=max(_env_float("FISH_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region="",
    ),
    "azure_speech": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint="",
        api_key=os.getenv("AZURE_SPEECH_API_KEY", "").strip(),
        timeout_sec=max(_env_float("AZURE_SPEECH_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region=os.getenv("AZURE_SPEECH_REGION", "").strip(),
    ),
    "elevenlabs": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint=(os.getenv("ELEVENLABS_TTS_ENDPOINT", "https://api.elevenlabs.io").strip() or "https://api.elevenlabs.io").rstrip("/"),
        api_key=os.getenv("ELEVENLABS_API_KEY", "").strip(),
        timeout_sec=max(_env_float("ELEVENLABS_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region="",
    ),
    "openai": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint=(os.getenv("OPENAI_TTS_ENDPOINT", "https://api.openai.com").strip() or "https://api.openai.com").rstrip("/"),
        api_key=os.getenv("OPENAI_API_KEY", "").strip(),
        timeout_sec=max(_env_float("OPENAI_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region="",
    ),
    "cartesia": lambda: SingleSpeakerTTSProviderRuntimeMetadata(
        endpoint=(os.getenv("CARTESIA_TTS_ENDPOINT", "https://api.cartesia.ai").strip() or "https://api.cartesia.ai").rstrip("/"),
        api_key=os.getenv("CARTESIA_API_KEY", "").strip(),
        timeout_sec=max(_env_float("CARTESIA_TTS_TIMEOUT_SEC", 300.0), 1.0),
        region="",
    ),
}


def load_single_speaker_tts_provider_runtime_metadata(provider: str) -> SingleSpeakerTTSProviderRuntimeMetadata:
    normalized_provider = (provider or "").strip().lower()
    loader = _SINGLE_SPEAKER_TTS_METADATA.get(normalized_provider)
    if loader:
        return loader()
    raise RuntimeError(f"unsupported single-speaker tts provider metadata: {provider}")
