import io
import json
import math
import os
from functools import lru_cache
from pathlib import Path

import httpx
try:
    import google.auth
    from google.auth.transport.requests import Request as GoogleAuthRequest
except ImportError:  # pragma: no cover - verified in runtime environments with dependency installed
    google = None
    GoogleAuthRequest = None
else:  # pragma: no cover - import side effect only
    google = google.auth

_GEMINI_TTS_SAMPLE_RATE = 48000
_GEMINI_TTS_CHANNELS = 2
_GEMINI_TTS_CONTENT_TYPE = "audio/mpeg"
_GEMINI_TTS_SUFFIX = ".mp3"


def _resolve_persona_file() -> Path:
    explicit = str(os.getenv("NAVIGATOR_PERSONAS_PATH") or "").strip()
    if explicit:
        return Path(explicit)
    llm_catalog = str(os.getenv("LLM_CATALOG_PATH") or "").strip()
    if llm_catalog:
        return Path(llm_catalog).resolve().parent / "ai_navigator_personas.json"
    return Path(__file__).resolve().parents[2] / "shared" / "ai_navigator_personas.json"


@lru_cache(maxsize=1)
def _load_persona_profiles() -> dict[str, dict]:
    persona_file = _resolve_persona_file()
    with persona_file.open("r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def resolve_audio_briefing_persona_prompts(persona: str) -> tuple[str, str, str]:
    persona_key = str(persona or "editor").strip() or "editor"
    profiles = _load_persona_profiles()
    profile = profiles.get(persona_key) or profiles.get("editor") or {}
    audio = profile.get("audio_briefing") or {}
    tone_prompt = str(audio.get("tone_prompt") or "").strip()
    speaking_style_prompt = str(audio.get("speaking_style_prompt") or "").strip()
    duo_conversation_prompt = str(audio.get("duo_conversation_prompt") or "").strip()
    return tone_prompt, speaking_style_prompt, duo_conversation_prompt


def build_gemini_tts_prompt(text: str, speech_rate: float) -> str:
    prompt_lines: list[str] = [
        "以下の本文を自然な日本語で読み上げてください。",
    ]
    if speech_rate > 0:
        if abs(speech_rate - 1.0) >= 0.05:
            prompt_lines.append(f"読み上げ速度は通常の約{speech_rate:.2f}倍を目安に、自然さを保ってください。")
        else:
            prompt_lines.append("読み上げ速度は自然な標準速度を維持してください。")
    prompt_lines.extend(
        [
            "追加の説明や要約は入れないでください。",
            "",
            (text or "").strip(),
        ]
    )
    return "\n".join(line for line in prompt_lines if line is not None)


def build_gemini_audio_briefing_prompt(persona: str, text: str, speech_rate: float) -> str:
    tone_prompt, speaking_style_prompt, _duo_conversation_prompt = resolve_audio_briefing_persona_prompts(persona)
    prompt_lines: list[str] = [
        "あなたは音声ブリーフィング番組を担当するAIナビゲーターです。",
        "以下のペルソナ指示を反映しつつ、日本語で自然に読み上げてください。",
    ]
    if tone_prompt:
        prompt_lines.append(f"トーン指示: {tone_prompt}")
    if speaking_style_prompt:
        prompt_lines.append(f"話し方指示: {speaking_style_prompt}")
    if speech_rate > 0:
        if abs(speech_rate - 1.0) >= 0.05:
            prompt_lines.append(f"読み上げ速度は通常の約{speech_rate:.2f}倍を目安に、自然さを保ってください。")
        else:
            prompt_lines.append("読み上げ速度は自然な標準速度を維持してください。")
    prompt_lines.extend(
        [
            "原稿は改変しないでください。追加説明や要約は入れないでください。",
            "",
            (text or "").strip(),
        ]
    )
    return "\n".join(line for line in prompt_lines if line is not None)


def build_gemini_duo_audio_briefing_prompt(
    host_persona: str,
    partner_persona: str,
    turns: list[dict[str, str]],
    section_type: str,
) -> str:
    host_tone_prompt, host_speaking_style_prompt, host_duo_prompt = resolve_audio_briefing_persona_prompts(host_persona)
    partner_tone_prompt, partner_speaking_style_prompt, partner_duo_prompt = resolve_audio_briefing_persona_prompts(partner_persona)

    section_label = {
        "opening": "オープニング",
        "summary": "総括",
        "article": "記事セクション",
        "ending": "エンディング",
    }.get(str(section_type or "").strip(), "会話セクション")
    dialogue_lines: list[str] = []
    for turn in turns:
        speaker = "HOST" if str(turn.get("speaker") or "").strip() == "host" else "PARTNER"
        text = str(turn.get("text") or "").strip()
        if not text:
            continue
        dialogue_lines.append(f"{speaker}: {text}")

    prompt_lines: list[str] = [
        "あなたは音声ブリーフィング番組の二人会話を音声化するAIです。",
        f"以下は {section_label} の台本です。会話の本文は改変せず、日本語の自然な掛け合いとして読み上げてください。",
        "話者ラベルは HOST と PARTNER を使います。",
    ]
    if host_tone_prompt:
        prompt_lines.append(f"HOST のトーン指示: {host_tone_prompt}")
    if host_speaking_style_prompt:
        prompt_lines.append(f"HOST の話し方指示: {host_speaking_style_prompt}")
    if host_duo_prompt:
        prompt_lines.append(f"HOST の会話指示: {host_duo_prompt}")
    if partner_tone_prompt:
        prompt_lines.append(f"PARTNER のトーン指示: {partner_tone_prompt}")
    if partner_speaking_style_prompt:
        prompt_lines.append(f"PARTNER の話し方指示: {partner_speaking_style_prompt}")
    if partner_duo_prompt:
        prompt_lines.append(f"PARTNER の会話指示: {partner_duo_prompt}")
    prompt_lines.extend(
        [
            "追加の説明や要約は入れないでください。",
            "",
            *dialogue_lines,
        ]
    )
    return "\n".join(line for line in prompt_lines if line is not None)


def _resolve_gemini_tts_endpoint() -> str:
    return (os.getenv("GEMINI_TTS_ENDPOINT", "https://texttospeech.googleapis.com").strip() or "https://texttospeech.googleapis.com").rstrip("/")


def _resolve_google_cloud_project_id(default_project_id: str | None) -> str:
    for env_name in ("GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "GOOGLE_PROJECT_ID"):
        value = str(os.getenv(env_name) or "").strip()
        if value:
            return value
    if default_project_id:
        return default_project_id
    raise RuntimeError("google cloud project id is not configured")


@lru_cache(maxsize=1)
def _cloud_tts_credentials():
    if google is None or GoogleAuthRequest is None:
        raise RuntimeError("google-auth is required for gemini cloud tts")
    creds, project_id = google.default(scopes=["https://www.googleapis.com/auth/cloud-platform"])
    return creds, project_id


def _cloud_tts_headers() -> dict[str, str]:
    creds, discovered_project_id = _cloud_tts_credentials()
    if not creds.valid or creds.expired or not creds.token:
        creds.refresh(GoogleAuthRequest())
    project_id = _resolve_google_cloud_project_id(discovered_project_id)
    return {
        "Authorization": f"Bearer {creds.token}",
        "x-goog-user-project": project_id,
        "Content-Type": "application/json",
    }


def _gemini_audio_bytes(payload: dict) -> bytes:
    data = str(payload.get("audioContent") or "").strip()
    if not data:
        raise RuntimeError("gemini cloud tts response did not include audio data")
    import base64
    return base64.b64decode(data.encode("utf-8"), validate=True)


def _estimate_mp3_duration_sec(audio_bytes: bytes) -> int:
    try:
        from mutagen.mp3 import MP3

        info = MP3(io.BytesIO(audio_bytes)).info
        length = float(getattr(info, "length", 0.0) or 0.0)
        if length > 0:
            return max(1, int(math.ceil(length)))
    except Exception:
        pass
    return 1


def synthesize_gemini_cloud_tts(
    *,
    model: str,
    voice_name: str,
    prompt: str,
    text: str,
) -> tuple[bytes, str, str, int]:
    normalized_model = (model or "").strip()
    normalized_voice_name = (voice_name or "").strip()
    normalized_text = (text or "").strip()
    if not normalized_model:
        raise RuntimeError("gemini tts model is required")
    if not normalized_voice_name:
        raise RuntimeError("gemini voice name is required")
    if not normalized_text:
        raise RuntimeError("gemini tts text is empty")
    response = httpx.post(
        f"{_resolve_gemini_tts_endpoint()}/v1/text:synthesize",
        headers=_cloud_tts_headers(),
        json={
            "input": {
                "text": normalized_text,
                "prompt": prompt,
            },
            "voice": {
                "languageCode": "ja-JP",
                "name": normalized_voice_name,
                "modelName": normalized_model,
            },
            "audioConfig": {
                "audioEncoding": "MP3",
                "sampleRateHertz": _GEMINI_TTS_SAMPLE_RATE,
            },
        },
        timeout=max(float(os.getenv("GEMINI_TTS_TIMEOUT_SEC", "300") or "300"), 1.0),
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        body = response.text.strip()
        if body:
            raise RuntimeError(f"gemini cloud tts request failed: status={response.status_code} body={body[:1000]}") from exc
        raise

    payload = response.json() if response.content else {}
    audio_bytes = _gemini_audio_bytes(payload)
    duration_sec = _estimate_mp3_duration_sec(audio_bytes)
    return audio_bytes, _GEMINI_TTS_CONTENT_TYPE, _GEMINI_TTS_SUFFIX, duration_sec


def synthesize_gemini_tts(
    *,
    model: str,
    voice_name: str,
    persona: str = "",
    text: str,
    speech_rate: float,
) -> tuple[bytes, str, str, int]:
    if str(persona or "").strip():
        prompt = build_gemini_audio_briefing_prompt(persona, text, speech_rate)
    else:
        prompt = build_gemini_tts_prompt(text, speech_rate)
    return synthesize_gemini_cloud_tts(
        model=model,
        voice_name=voice_name,
        prompt=prompt,
        text=text,
    )


def synthesize_gemini_multi_speaker_tts(
    *,
    model: str,
    host_voice_name: str,
    partner_voice_name: str,
    host_persona: str,
    partner_persona: str,
    section_type: str,
    turns: list[dict[str, str]],
) -> tuple[bytes, str, str, int]:
    normalized_model = (model or "").strip()
    normalized_host_voice_name = (host_voice_name or "").strip()
    normalized_partner_voice_name = (partner_voice_name or "").strip()
    filtered_turns = []
    for turn in turns or []:
        speaker = str((turn or {}).get("speaker") or "").strip()
        text = str((turn or {}).get("text") or "").strip()
        if speaker not in {"host", "partner"} or not text:
            continue
        filtered_turns.append({"speaker": speaker, "text": text})

    if not normalized_model:
        raise RuntimeError("gemini tts model is required")
    if not normalized_host_voice_name:
        raise RuntimeError("gemini host voice name is required")
    if not normalized_partner_voice_name:
        raise RuntimeError("gemini partner voice name is required")
    if not filtered_turns:
        raise RuntimeError("gemini duo turns are empty")

    prompt = build_gemini_duo_audio_briefing_prompt(host_persona, partner_persona, filtered_turns, section_type)
    response = httpx.post(
        f"{_resolve_gemini_tts_endpoint()}/v1/text:synthesize",
        headers=_cloud_tts_headers(),
        json={
            "input": {
                "prompt": prompt,
                "multiSpeakerMarkup": {
                    "turns": [
                        {
                            "speaker": "HOST" if turn["speaker"] == "host" else "PARTNER",
                            "text": turn["text"],
                        }
                        for turn in filtered_turns
                    ]
                },
            },
            "voice": {
                "languageCode": "ja-JP",
                "modelName": normalized_model,
                "multiSpeakerVoiceConfig": {
                    "speakerVoiceConfigs": [
                        {
                            "speakerAlias": "HOST",
                            "speakerId": normalized_host_voice_name,
                        },
                        {
                            "speakerAlias": "PARTNER",
                            "speakerId": normalized_partner_voice_name,
                        },
                    ]
                },
            },
            "audioConfig": {
                "audioEncoding": "MP3",
                "sampleRateHertz": _GEMINI_TTS_SAMPLE_RATE,
            },
        },
        timeout=max(float(os.getenv("GEMINI_TTS_TIMEOUT_SEC", "300") or "300"), 1.0),
    )
    try:
        response.raise_for_status()
    except httpx.HTTPStatusError as exc:
        body = response.text.strip()
        if body:
            raise RuntimeError(f"gemini cloud duo tts request failed: status={response.status_code} body={body[:1000]}") from exc
        raise

    payload = response.json() if response.content else {}
    audio_bytes = _gemini_audio_bytes(payload)
    duration_sec = _estimate_mp3_duration_sec(audio_bytes)
    return audio_bytes, _GEMINI_TTS_CONTENT_TYPE, _GEMINI_TTS_SUFFIX, duration_sec
