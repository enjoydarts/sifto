import base64
import os

from app.services.audio_briefing_tts import _env_float
from app.services.audio_briefing_tts import synthesize_mock_audio
from app.services.aivis_speech import AivisSpeechService
from app.services.elevenlabs_tts import synthesize_elevenlabs_tts
from app.services.fish_tts import synthesize_fish_tts
from app.services.gemini_tts import synthesize_gemini_tts
from app.services.tts_provider_registry import synthesize_catalog_tts


def build_summary_audio_text(translated_title: str | None, original_title: str | None, summary: str) -> str:
    title = (translated_title or "").strip() or (original_title or "").strip()
    summary_text = (summary or "").strip()
    if not title:
        raise RuntimeError("summary audio title is empty")
    if not summary_text:
        raise RuntimeError("summary audio summary is empty")
    return f"{title}\n\n{summary_text}"


class SummaryAudioPlayerService:
    def __init__(self) -> None:
        self.aivis = AivisSpeechService()
        self.xai_tts_endpoint = (os.getenv("XAI_TTS_ENDPOINT", "https://api.x.ai").strip() or "https://api.x.ai").rstrip("/")
        self.xai_timeout_sec = max(_env_float("XAI_TTS_TIMEOUT_SEC", 300.0), 1.0)
        self.gemini_tts_endpoint = (os.getenv("GEMINI_TTS_ENDPOINT", "https://generativelanguage.googleapis.com").strip() or "https://generativelanguage.googleapis.com").rstrip("/")
        self.gemini_timeout_sec = max(_env_float("GEMINI_TTS_TIMEOUT_SEC", 300.0), 1.0)
        self.fish_api_key = os.getenv("FISH_API_KEY", "").strip()
        self.fish_timeout_sec = max(_env_float("FISH_TTS_TIMEOUT_SEC", 300.0), 1.0)
        self.elevenlabs_tts_endpoint = (os.getenv("ELEVENLABS_TTS_ENDPOINT", "https://api.elevenlabs.io").strip() or "https://api.elevenlabs.io").rstrip("/")
        self.elevenlabs_api_key = os.getenv("ELEVENLABS_API_KEY", "").strip()
        self.elevenlabs_timeout_sec = max(_env_float("ELEVENLABS_TTS_TIMEOUT_SEC", 300.0), 1.0)
        self.openai_tts_endpoint = (os.getenv("OPENAI_TTS_ENDPOINT", "https://api.openai.com").strip() or "https://api.openai.com").rstrip("/")
        self.openai_timeout_sec = max(_env_float("OPENAI_TTS_TIMEOUT_SEC", 300.0), 1.0)

    def synthesize(
        self,
        *,
        provider: str,
        voice_model: str,
        voice_style: str,
        text: str,
        speech_rate: float,
        emotional_intensity: float,
        tempo_dynamics: float,
        line_break_silence_seconds: float,
        chunk_trailing_silence_seconds: float,
        pitch: float,
        volume_gain: float,
        tts_model: str = "",
        user_dictionary_uuid: str | None = None,
        aivis_api_key: str | None = None,
        google_api_key: str | None = None,
        fish_api_key: str | None = None,
        elevenlabs_api_key: str | None = None,
        xai_api_key: str | None = None,
        openai_api_key: str | None = None,
    ) -> tuple[str, str, int, str]:
        normalized_provider = (provider or "").strip().lower()
        if normalized_provider == "mock":
            audio_bytes, content_type, _, duration_sec = synthesize_mock_audio(text, speech_rate)
        elif normalized_provider == "aivis":
            audio_bytes, content_type, _, duration_sec = self.aivis.synthesize(
                voice_model=voice_model,
                voice_style=voice_style,
                text=text,
                speech_rate=speech_rate,
                emotional_intensity=emotional_intensity,
                tempo_dynamics=tempo_dynamics,
                line_break_silence_seconds=line_break_silence_seconds,
                chunk_trailing_silence_seconds=chunk_trailing_silence_seconds,
                pitch=pitch,
                volume_gain=volume_gain,
                user_dictionary_uuid=user_dictionary_uuid,
                api_key_override=aivis_api_key,
            )
        elif normalized_provider == "xai":
            audio_bytes, content_type, _, duration_sec = synthesize_catalog_tts(
                "xai",
                endpoint=self.xai_tts_endpoint,
                api_key=xai_api_key or "",
                voice_id=voice_model,
                tts_model="",
                text=text,
                speech_rate=speech_rate,
                timeout_sec=self.xai_timeout_sec,
            )
        elif normalized_provider == "gemini_tts":
            audio_bytes, content_type, _, duration_sec = synthesize_gemini_tts(
                model=tts_model,
                voice_name=voice_model,
                text=text,
                speech_rate=speech_rate,
                api_key=google_api_key,
            )
        elif normalized_provider == "fish":
            audio_bytes, content_type, _, duration_sec = synthesize_fish_tts(
                model=tts_model,
                voice_name=voice_model,
                text=text,
                speech_rate=speech_rate,
                volume_gain=volume_gain,
                api_key=(fish_api_key or "").strip() or self.fish_api_key,
                timeout_sec=self.fish_timeout_sec,
            )
        elif normalized_provider == "elevenlabs":
            audio_bytes, content_type, _, duration_sec = synthesize_elevenlabs_tts(
                endpoint=self.elevenlabs_tts_endpoint,
                api_key=(elevenlabs_api_key or "").strip() or self.elevenlabs_api_key,
                model=tts_model,
                voice_id=voice_model,
                text=text,
                timeout_sec=self.elevenlabs_timeout_sec,
            )
        elif normalized_provider == "openai":
            audio_bytes, content_type, _, duration_sec = synthesize_catalog_tts(
                "openai",
                endpoint=self.openai_tts_endpoint,
                api_key=openai_api_key or "",
                voice_id=voice_model,
                tts_model=tts_model,
                text=text,
                speech_rate=speech_rate,
                timeout_sec=self.openai_timeout_sec,
            )
        else:
            raise RuntimeError(f"unsupported summary audio provider: {provider}")
        return base64.b64encode(audio_bytes).decode("utf-8"), content_type, duration_sec, text
