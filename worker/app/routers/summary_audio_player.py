from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.summary_audio_player import SummaryAudioPlayerService

router = APIRouter()


class SummaryAudioSynthesizeRequest(BaseModel):
    provider: str
    voice_model: str
    voice_style: str
    tts_model: str = ""
    text: str
    persona: str = ""
    speech_rate: float = 1.0
    emotional_intensity: float = 1.0
    tempo_dynamics: float = 1.0
    line_break_silence_seconds: float = 0.4
    chunk_trailing_silence_seconds: float = 1.0
    pitch: float = 0.0
    volume_gain: float = 0.0
    user_dictionary_uuid: str | None = None


class SummaryAudioSynthesizeResponse(BaseModel):
    audio_base64: str
    content_type: str
    duration_sec: int
    resolved_text: str


@router.post("/summary-audio/synthesize", response_model=SummaryAudioSynthesizeResponse)
def synthesize_summary_audio(req: SummaryAudioSynthesizeRequest, request: Request):
    try:
        service = SummaryAudioPlayerService()
        aivis_api_key = request.headers.get("x-aivis-api-key", "").strip() or None
        google_api_key = request.headers.get("x-google-api-key", "").strip() or None
        xai_api_key = request.headers.get("x-xai-api-key", "").strip() or None
        audio_base64, content_type, duration_sec, resolved_text = service.synthesize(
            provider=req.provider,
            voice_model=req.voice_model,
            voice_style=req.voice_style,
            tts_model=req.tts_model,
            text=req.text,
            persona=req.persona,
            speech_rate=req.speech_rate,
            emotional_intensity=req.emotional_intensity,
            tempo_dynamics=req.tempo_dynamics,
            line_break_silence_seconds=req.line_break_silence_seconds,
            chunk_trailing_silence_seconds=req.chunk_trailing_silence_seconds,
            pitch=req.pitch,
            volume_gain=req.volume_gain,
            user_dictionary_uuid=req.user_dictionary_uuid,
            aivis_api_key=aivis_api_key,
            google_api_key=google_api_key,
            xai_api_key=xai_api_key,
            openai_api_key=request.headers.get("x-openai-api-key", "").strip() or None,
        )
        return SummaryAudioSynthesizeResponse(
            audio_base64=audio_base64,
            content_type=content_type,
            duration_sec=duration_sec,
            resolved_text=resolved_text,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"summary audio synth failed: {exc}")
