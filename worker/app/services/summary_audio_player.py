import base64

from app.services.audio_briefing_tts import synthesize_mock_audio
from app.services.aivis_speech import AivisSpeechService


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
        pitch: float,
        volume_gain: float,
        user_dictionary_uuid: str | None = None,
        aivis_api_key: str | None = None,
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
                pitch=pitch,
                volume_gain=volume_gain,
                user_dictionary_uuid=user_dictionary_uuid,
                api_key_override=aivis_api_key,
            )
        else:
            raise RuntimeError(f"unsupported summary audio provider: {provider}")
        return base64.b64encode(audio_bytes).decode("utf-8"), content_type, duration_sec, text
