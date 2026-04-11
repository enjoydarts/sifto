import base64
import os

from app.services.audio_briefing_tts import synthesize_mock_audio
from app.services.aivis_speech import AivisSpeechService
from app.services.tts_provider_metadata import _env_float
from app.services.tts_provider_metadata import load_single_speaker_tts_provider_runtime_metadata
from app.services.tts_provider_registry import synthesize_single_speaker_tts


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
        self.single_speaker_provider_runtime = {
            provider: load_single_speaker_tts_provider_runtime_metadata(provider)
            for provider in ("xai", "gemini_tts", "fish", "elevenlabs", "openai", "azure_speech")
        }
        self.gemini_tts_endpoint = self.single_speaker_provider_runtime["gemini_tts"].endpoint
        self.gemini_timeout_sec = self.single_speaker_provider_runtime["gemini_tts"].timeout_sec
        self.fish_api_key = self.single_speaker_provider_runtime["fish"].api_key
        self.fish_timeout_sec = self.single_speaker_provider_runtime["fish"].timeout_sec
        self.elevenlabs_tts_endpoint = self.single_speaker_provider_runtime["elevenlabs"].endpoint
        self.elevenlabs_api_key = self.single_speaker_provider_runtime["elevenlabs"].api_key
        self.elevenlabs_timeout_sec = self.single_speaker_provider_runtime["elevenlabs"].timeout_sec

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
        azure_speech_api_key: str | None = None,
        azure_speech_region: str | None = None,
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
        elif normalized_provider in {"xai", "gemini_tts", "fish", "elevenlabs", "openai", "azure_speech"}:
            runtime = self.single_speaker_provider_runtime[normalized_provider]
            api_key = runtime.api_key
            region = runtime.region
            if normalized_provider == "xai":
                api_key = (xai_api_key or "").strip() or api_key
            elif normalized_provider == "gemini_tts":
                api_key = (google_api_key or "").strip() or api_key
            elif normalized_provider == "fish":
                api_key = (fish_api_key or "").strip() or api_key
            elif normalized_provider == "elevenlabs":
                api_key = (elevenlabs_api_key or "").strip() or api_key
            elif normalized_provider == "openai":
                api_key = (openai_api_key or "").strip() or api_key
            elif normalized_provider == "azure_speech":
                api_key = (azure_speech_api_key or "").strip() or api_key
                region = (azure_speech_region or "").strip() or region
            audio_bytes, content_type, _, duration_sec = synthesize_single_speaker_tts(
                normalized_provider,
                endpoint=runtime.endpoint,
                api_key=api_key,
                region=region,
                voice_id=voice_model,
                tts_model=tts_model,
                text=text,
                speech_rate=speech_rate,
                timeout_sec=runtime.timeout_sec,
                volume_gain=volume_gain,
                line_break_silence_seconds=line_break_silence_seconds,
                pitch=pitch,
            )
        else:
            raise RuntimeError(f"unsupported summary audio provider: {provider}")
        return base64.b64encode(audio_bytes).decode("utf-8"), content_type, duration_sec, text
