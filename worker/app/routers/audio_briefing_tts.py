from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel

from app.services.audio_briefing_tts import AudioBriefingTTSService

router = APIRouter()


class AudioBriefingSynthesizeRequest(BaseModel):
    provider: str
    voice_model: str
    voice_style: str
    persona: str = ""
    tts_model: str = ""
    text: str
    speech_rate: float = 1.0
    emotional_intensity: float = 1.0
    tempo_dynamics: float = 1.0
    line_break_silence_seconds: float = 0.4
    chunk_trailing_silence_seconds: float = 1.0
    pitch: float = 0.0
    volume_gain: float = 0.0
    output_object_key: str
    chunk_id: str | None = None
    heartbeat_url: str | None = None
    heartbeat_token: str | None = None
    user_dictionary_uuid: str | None = None


class AudioBriefingSynthesizeResponse(BaseModel):
    audio_object_key: str
    duration_sec: int


class AudioBriefingGeminiDuoTurn(BaseModel):
    speaker: str
    text: str


class AudioBriefingGeminiDuoSynthesizeRequest(BaseModel):
    tts_model: str
    host_persona: str
    partner_persona: str
    host_voice_model: str
    partner_voice_model: str
    section_type: str
    turns: list[AudioBriefingGeminiDuoTurn]
    preprocessed_text: str | None = None
    output_object_key: str


class AudioBriefingPresignRequest(BaseModel):
    object_key: str
    expires_sec: int = 3600
    bucket: str | None = None


class AudioBriefingPresignResponse(BaseModel):
    audio_url: str


class AudioBriefingDeleteObjectsRequest(BaseModel):
    object_keys: list[str]
    bucket: str | None = None


class AudioBriefingDeleteObjectsResponse(BaseModel):
    deleted_count: int


class AudioBriefingCopyObjectsRequest(BaseModel):
    source_bucket: str
    target_bucket: str
    object_keys: list[str]


class AudioBriefingCopyObjectsResponse(BaseModel):
    copied_count: int


class AudioBriefingStatObjectRequest(BaseModel):
    object_key: str
    bucket: str | None = None


class AudioBriefingStatObjectResponse(BaseModel):
    size_bytes: int


class AudioBriefingUploadObjectRequest(BaseModel):
    object_key: str
    content_base64: str
    content_type: str
    bucket: str | None = None


class AudioBriefingUploadObjectResponse(BaseModel):
    object_key: str


@router.post("/audio-briefing/synthesize-upload", response_model=AudioBriefingSynthesizeResponse)
def synthesize_audio_briefing(req: AudioBriefingSynthesizeRequest, request: Request):
    try:
        service = AudioBriefingTTSService()
        aivis_api_key = request.headers.get("x-aivis-api-key", "").strip() or None
        google_api_key = request.headers.get("x-google-api-key", "").strip() or None
        fish_api_key = request.headers.get("x-fish-api-key", "").strip() or None
        elevenlabs_api_key = request.headers.get("x-elevenlabs-api-key", "").strip() or None
        xai_api_key = request.headers.get("x-xai-api-key", "").strip() or None
        audio_object_key, duration_sec = service.synthesize_and_upload(
            provider=req.provider,
            voice_model=req.voice_model,
            voice_style=req.voice_style,
            persona=req.persona,
            tts_model=req.tts_model,
            text=req.text,
            speech_rate=req.speech_rate,
            emotional_intensity=req.emotional_intensity,
            tempo_dynamics=req.tempo_dynamics,
            line_break_silence_seconds=req.line_break_silence_seconds,
            chunk_trailing_silence_seconds=req.chunk_trailing_silence_seconds,
            pitch=req.pitch,
            volume_gain=req.volume_gain,
            output_object_key=req.output_object_key,
            chunk_id=req.chunk_id,
            heartbeat_url=req.heartbeat_url,
            heartbeat_token=req.heartbeat_token,
            user_dictionary_uuid=req.user_dictionary_uuid,
            aivis_api_key=aivis_api_key,
            google_api_key=google_api_key,
            fish_api_key=fish_api_key,
            elevenlabs_api_key=elevenlabs_api_key,
            xai_api_key=xai_api_key,
            openai_api_key=request.headers.get("x-openai-api-key", "").strip() or None,
        )
        return AudioBriefingSynthesizeResponse(
            audio_object_key=audio_object_key,
            duration_sec=duration_sec,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing synth failed: {exc}")


@router.post("/audio-briefing/synthesize-upload-gemini-duo", response_model=AudioBriefingSynthesizeResponse)
def synthesize_audio_briefing_gemini_duo(req: AudioBriefingGeminiDuoSynthesizeRequest, request: Request):
    try:
        service = AudioBriefingTTSService()
        audio_object_key, duration_sec = service.synthesize_gemini_duo_and_upload(
            tts_model=req.tts_model,
            host_persona=req.host_persona,
            partner_persona=req.partner_persona,
            host_voice_model=req.host_voice_model,
            partner_voice_model=req.partner_voice_model,
            section_type=req.section_type,
            turns=[turn.model_dump() for turn in req.turns],
            output_object_key=req.output_object_key,
            api_key_override=request.headers.get("x-google-api-key", "").strip() or None,
        )
        return AudioBriefingSynthesizeResponse(
            audio_object_key=audio_object_key,
            duration_sec=duration_sec,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing gemini duo synth failed: {exc}")


@router.post("/audio-briefing/synthesize-upload-fish-duo", response_model=AudioBriefingSynthesizeResponse)
def synthesize_audio_briefing_fish_duo(req: AudioBriefingGeminiDuoSynthesizeRequest, request: Request):
    try:
        service = AudioBriefingTTSService()
        audio_object_key, duration_sec = service.synthesize_fish_duo_and_upload(
            tts_model=req.tts_model,
            host_persona=req.host_persona,
            partner_persona=req.partner_persona,
            host_voice_model=req.host_voice_model,
            partner_voice_model=req.partner_voice_model,
            section_type=req.section_type,
            turns=[turn.model_dump() for turn in req.turns],
            preprocessed_text=req.preprocessed_text,
            output_object_key=req.output_object_key,
            api_key_override=request.headers.get("x-fish-api-key", "").strip() or None,
        )
        return AudioBriefingSynthesizeResponse(
            audio_object_key=audio_object_key,
            duration_sec=duration_sec,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing fish duo synth failed: {exc}")


@router.post("/audio-briefing/synthesize-upload-elevenlabs-duo", response_model=AudioBriefingSynthesizeResponse)
def synthesize_audio_briefing_elevenlabs_duo(req: AudioBriefingGeminiDuoSynthesizeRequest, request: Request):
    try:
        service = AudioBriefingTTSService()
        audio_object_key, duration_sec = service.synthesize_elevenlabs_duo_and_upload(
            tts_model=req.tts_model,
            host_persona=req.host_persona,
            partner_persona=req.partner_persona,
            host_voice_model=req.host_voice_model,
            partner_voice_model=req.partner_voice_model,
            section_type=req.section_type,
            turns=[turn.model_dump() for turn in req.turns],
            output_object_key=req.output_object_key,
            api_key_override=request.headers.get("x-elevenlabs-api-key", "").strip() or None,
        )
        return AudioBriefingSynthesizeResponse(
            audio_object_key=audio_object_key,
            duration_sec=duration_sec,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing elevenlabs duo synth failed: {exc}")


@router.post("/audio-briefing/presign", response_model=AudioBriefingPresignResponse)
def presign_audio_briefing(req: AudioBriefingPresignRequest):
    try:
        service = AudioBriefingTTSService()
        audio_url = service.presign_audio_url(
            object_key=req.object_key,
            expires_sec=req.expires_sec,
            bucket_override=req.bucket,
        )
        return AudioBriefingPresignResponse(audio_url=audio_url)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing presign failed: {exc}")


@router.post("/audio-briefing/delete-objects", response_model=AudioBriefingDeleteObjectsResponse)
def delete_audio_briefing_objects(req: AudioBriefingDeleteObjectsRequest):
    try:
        service = AudioBriefingTTSService()
        deleted_count = service.delete_objects(req.object_keys, bucket_override=req.bucket)
        return AudioBriefingDeleteObjectsResponse(deleted_count=deleted_count)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing delete failed: {exc}")


@router.post("/audio-briefing/copy-objects", response_model=AudioBriefingCopyObjectsResponse)
def copy_audio_briefing_objects(req: AudioBriefingCopyObjectsRequest):
    try:
        service = AudioBriefingTTSService()
        copied_count = service.copy_objects(
            source_bucket=req.source_bucket,
            target_bucket=req.target_bucket,
            object_keys=req.object_keys,
        )
        return AudioBriefingCopyObjectsResponse(copied_count=copied_count)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing copy failed: {exc}")


@router.post("/audio-briefing/stat-object", response_model=AudioBriefingStatObjectResponse)
def stat_audio_briefing_object(req: AudioBriefingStatObjectRequest):
    try:
        service = AudioBriefingTTSService()
        size_bytes = service.stat_object(req.object_key, bucket_override=req.bucket)
        return AudioBriefingStatObjectResponse(size_bytes=size_bytes)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing stat failed: {exc}")


@router.post("/audio-briefing/upload-object", response_model=AudioBriefingUploadObjectResponse)
def upload_audio_briefing_object(req: AudioBriefingUploadObjectRequest):
    try:
        service = AudioBriefingTTSService()
        object_key = service.upload_base64_object(
            object_key=req.object_key,
            content_base64=req.content_base64,
            content_type=req.content_type,
            bucket_override=req.bucket,
        )
        return AudioBriefingUploadObjectResponse(object_key=object_key)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"audio briefing upload failed: {exc}")
