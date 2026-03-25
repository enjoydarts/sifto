from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.audio_briefing_tts import AudioBriefingTTSService

router = APIRouter()


class AudioBriefingSynthesizeRequest(BaseModel):
    provider: str
    voice_model: str
    voice_style: str
    text: str
    speech_rate: float = 1.0
    emotional_intensity: float = 1.0
    tempo_dynamics: float = 1.0
    line_break_silence_seconds: float = 0.4
    pitch: float = 0.0
    volume_gain: float = 0.0
    output_object_key: str


class AudioBriefingSynthesizeResponse(BaseModel):
    audio_object_key: str
    duration_sec: int


class AudioBriefingPresignRequest(BaseModel):
    object_key: str
    expires_sec: int = 3600


class AudioBriefingPresignResponse(BaseModel):
    audio_url: str


@router.post("/audio-briefing/synthesize-upload", response_model=AudioBriefingSynthesizeResponse)
def synthesize_audio_briefing(req: AudioBriefingSynthesizeRequest, request: Request):
    try:
        service = AudioBriefingTTSService()
        aivis_api_key = request.headers.get("x-aivis-api-key", "").strip() or None
        audio_object_key, duration_sec = service.synthesize_and_upload(
            provider=req.provider,
            voice_model=req.voice_model,
            voice_style=req.voice_style,
            text=req.text,
            speech_rate=req.speech_rate,
            emotional_intensity=req.emotional_intensity,
            tempo_dynamics=req.tempo_dynamics,
            line_break_silence_seconds=req.line_break_silence_seconds,
            pitch=req.pitch,
            volume_gain=req.volume_gain,
            output_object_key=req.output_object_key,
            aivis_api_key=aivis_api_key,
        )
        return AudioBriefingSynthesizeResponse(
            audio_object_key=audio_object_key,
            duration_sec=duration_sec,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing synth failed: {exc}")


@router.post("/audio-briefing/presign", response_model=AudioBriefingPresignResponse)
def presign_audio_briefing(req: AudioBriefingPresignRequest):
    try:
        service = AudioBriefingTTSService()
        audio_url = service.presign_audio_url(
            object_key=req.object_key,
            expires_sec=req.expires_sec,
        )
        return AudioBriefingPresignResponse(audio_url=audio_url)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing presign failed: {exc}")
