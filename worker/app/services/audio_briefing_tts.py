import io
import hashlib
import json
import math
import os
import threading
import time
import wave

import boto3
import httpx
from mutagen.mp3 import MP3
try:
    import redis
except Exception:  # pragma: no cover
    redis = None


class AivisRateLimiter:
    def __init__(self, min_interval_sec: float = 8.6) -> None:
        self.min_interval_sec = max(float(min_interval_sec or 0), 0.0)
        self._lock = threading.Lock()
        self._next_allowed_at: dict[str, float] = {}

    def acquire(self, key: str) -> None:
        if self.min_interval_sec <= 0:
            return
        key = (key or "__default__").strip() or "__default__"
        with self._lock:
            now = time.monotonic()
            next_allowed_at = self._next_allowed_at.get(key, 0.0)
            wait_sec = max(0.0, next_allowed_at - now)
            if wait_sec > 0:
                time.sleep(wait_sec)
                now += wait_sec
            self._next_allowed_at[key] = now + self.min_interval_sec


class AivisRedisRateLimiter:
    def __init__(
        self,
        redis_client,
        min_interval_sec: float = 8.6,
        fallback: AivisRateLimiter | None = None,
        max_wait_sec: float = 120.0,
    ) -> None:
        self.redis_client = redis_client
        self.min_interval_sec = max(float(min_interval_sec or 0), 0.0)
        self.fallback = fallback or AivisRateLimiter(min_interval_sec=min_interval_sec)
        self.max_wait_sec = max(float(max_wait_sec or 0), 0.0)

    def acquire(self, key: str) -> None:
        if self.redis_client is None or self.min_interval_sec <= 0:
            self.fallback.acquire(key)
            return
        ttl_ms = max(int(math.ceil(self.min_interval_sec * 1000)), 1)
        redis_key = self._redis_key(key)
        started_at = time.monotonic()
        while True:
            try:
                if self.redis_client.set(redis_key, "1", nx=True, px=ttl_ms):
                    return
                remaining_ms = self.redis_client.pttl(redis_key)
            except Exception:
                self.fallback.acquire(key)
                return
            if self.max_wait_sec > 0:
                elapsed_sec = time.monotonic() - started_at
                remaining_budget_sec = self.max_wait_sec - elapsed_sec
                if remaining_budget_sec <= 0:
                    self.fallback.acquire(key)
                    return
            else:
                remaining_budget_sec = 0.0
            wait_sec = self.min_interval_sec if remaining_ms is None or remaining_ms < 0 else max(float(remaining_ms) / 1000.0, 0.05)
            if self.max_wait_sec > 0:
                wait_sec = min(wait_sec, max(remaining_budget_sec, 0.05))
            time.sleep(wait_sec)

    def _redis_key(self, key: str) -> str:
        raw = (key or "__default__").strip() or "__default__"
        digest = hashlib.sha256(raw.encode("utf-8")).hexdigest()
        return f"audio-briefing:aivis-rate-limit:{digest}"


def _env_float(name: str, default: float) -> float:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def parse_retry_after_seconds(value: str | None, fallback: float) -> float:
    raw = (value or "").strip()
    if not raw:
        return fallback
    try:
        parsed = float(raw)
    except ValueError:
        return fallback
    return parsed if parsed > 0 else fallback


_REDIS_CLIENT = None


def redis_client():
    global _REDIS_CLIENT
    if _REDIS_CLIENT is not None:
        return _REDIS_CLIENT
    if redis is None:
        return None
    redis_url = os.getenv("REDIS_URL") or os.getenv("UPSTASH_REDIS_URL") or ""
    if not redis_url:
        return None
    try:
        _REDIS_CLIENT = redis.Redis.from_url(redis_url, decode_responses=True)
    except Exception:
        _REDIS_CLIENT = None
    return _REDIS_CLIENT


_AIVIS_MIN_INTERVAL_SEC = 60.0 / max(_env_float("AIVIS_TTS_REQUESTS_PER_MINUTE", 7.0), 1.0)
_AIVIS_REDIS_MAX_WAIT_SEC = max(_env_float("AIVIS_TTS_REDIS_MAX_WAIT_SEC", 120.0), 0.0)
_AIVIS_FALLBACK_RATE_LIMITER = AivisRateLimiter(min_interval_sec=_AIVIS_MIN_INTERVAL_SEC)
AIVIS_RATE_LIMITER = AivisRedisRateLimiter(
    redis_client=redis_client(),
    min_interval_sec=_AIVIS_MIN_INTERVAL_SEC,
    fallback=_AIVIS_FALLBACK_RATE_LIMITER,
    max_wait_sec=_AIVIS_REDIS_MAX_WAIT_SEC,
)


class AudioBriefingTTSService:
    def __init__(self) -> None:
        self.r2_endpoint = os.getenv("AUDIO_BRIEFING_R2_ENDPOINT", "").strip()
        self.r2_standard_bucket = (
            os.getenv("AUDIO_BRIEFING_R2_STANDARD_BUCKET", "").strip()
            or os.getenv("AUDIO_BRIEFING_R2_BUCKET", "").strip()
        )
        self.r2_bucket = self.r2_standard_bucket
        self.r2_ia_bucket = os.getenv("AUDIO_BRIEFING_R2_IA_BUCKET", "").strip()
        self.r2_region = os.getenv("AUDIO_BRIEFING_R2_REGION", "auto").strip() or "auto"
        self.r2_access_key_id = os.getenv("AUDIO_BRIEFING_R2_ACCESS_KEY_ID", "").strip()
        self.r2_secret_access_key = os.getenv("AUDIO_BRIEFING_R2_SECRET_ACCESS_KEY", "").strip()
        self.aivis_tts_endpoint = os.getenv("AIVIS_TTS_ENDPOINT", "").strip()
        self.aivis_api_key = os.getenv("AIVIS_API_KEY", "").strip()
        self.aivis_retry_attempts = max(_env_int("AIVIS_TTS_RETRY_ATTEMPTS", 2), 1)
        self.aivis_retry_fallback_sec = max(_env_float("AIVIS_TTS_RETRY_FALLBACK_SEC", 9.0), 0.0)
        self.aivis_timeout_sec = max(_env_float("AIVIS_TTS_TIMEOUT_SEC", 300.0), 1.0)

    def synthesize_and_upload(
        self,
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
        output_object_key: str,
        aivis_api_key: str | None = None,
    ) -> tuple[str, int]:
        provider = (provider or "").strip().lower()
        if provider == "mock":
            payload, content_type, suffix, duration_sec = synthesize_mock_audio(text, speech_rate)
        elif provider == "aivis":
            payload, content_type, suffix, duration_sec = self.synthesize_aivis_audio(
                voice_model=voice_model,
                voice_style=voice_style,
                text=text,
                speech_rate=speech_rate,
                emotional_intensity=emotional_intensity,
                tempo_dynamics=tempo_dynamics,
                line_break_silence_seconds=line_break_silence_seconds,
                pitch=pitch,
                volume_gain=volume_gain,
                api_key_override=aivis_api_key,
            )
        else:
            raise RuntimeError(f"unsupported tts provider: {provider}")

        if not output_object_key.endswith(suffix):
            output_object_key = output_object_key + suffix
        self.upload_bytes(output_object_key, payload, content_type)
        return output_object_key, duration_sec

    def standard_bucket(self) -> str:
        return (self.r2_bucket or self.r2_standard_bucket or "").strip()

    def ia_bucket(self) -> str:
        return (self.r2_ia_bucket or "").strip()

    def resolve_bucket(self, bucket_override: str | None = None) -> str:
        bucket = (bucket_override or "").strip() or self.standard_bucket()
        if not bucket:
            raise RuntimeError("audio briefing R2 bucket is not configured")
        return bucket

    def upload_bytes(self, object_key: str, payload: bytes, content_type: str, bucket_override: str | None = None) -> None:
        client = self.r2_client()
        client.put_object(
            Bucket=self.resolve_bucket(bucket_override),
            Key=object_key,
            Body=payload,
            ContentType=content_type,
        )

    def presign_audio_url(self, object_key: str, expires_sec: int = 3600, bucket_override: str | None = None) -> str:
        client = self.r2_client()
        return client.generate_presigned_url(
            "get_object",
            Params={
                "Bucket": self.resolve_bucket(bucket_override),
                "Key": object_key,
            },
            ExpiresIn=max(int(expires_sec), 60),
        )

    def delete_objects(self, object_keys: list[str], bucket_override: str | None = None) -> int:
        keys: list[str] = []
        seen: set[str] = set()
        for raw in object_keys or []:
            key = (raw or "").strip()
            if not key or key in seen:
                continue
            seen.add(key)
            keys.append(key)
        if not keys:
            return 0
        client = self.r2_client()
        bucket = self.resolve_bucket(bucket_override)
        deleted_count = 0
        for start in range(0, len(keys), 1000):
            batch = keys[start : start + 1000]
            result = client.delete_objects(
                Bucket=bucket,
                Delete={
                    "Objects": [{"Key": key} for key in batch],
                    "Quiet": True,
                },
            )
            errors = result.get("Errors") or []
            if errors:
                raise RuntimeError(f"R2 delete failed: {errors}")
            deleted_count += len(result.get("Deleted") or [])
        return deleted_count

    def copy_objects(self, source_bucket: str, target_bucket: str, object_keys: list[str]) -> int:
        source = self.resolve_bucket(source_bucket)
        target = self.resolve_bucket(target_bucket)
        keys: list[str] = []
        seen: set[str] = set()
        for raw in object_keys or []:
            key = (raw or "").strip()
            if not key or key in seen:
                continue
            seen.add(key)
            keys.append(key)
        if not keys:
            return 0
        client = self.r2_client()
        for key in keys:
            client.copy_object(
                Bucket=target,
                Key=key,
                CopySource={"Bucket": source, "Key": key},
            )
        return len(keys)

    def r2_client(self):
        if not self.r2_endpoint or not (self.standard_bucket() or self.ia_bucket()) or not self.r2_access_key_id or not self.r2_secret_access_key:
            raise RuntimeError("R2 settings are not configured")
        return boto3.client(
            "s3",
            endpoint_url=self.r2_endpoint,
            aws_access_key_id=self.r2_access_key_id,
            aws_secret_access_key=self.r2_secret_access_key,
            region_name=self.r2_region,
        )

    def synthesize_aivis_audio(
        self,
        *,
        voice_model: str,
        voice_style: str,
        text: str,
        speech_rate: float,
        emotional_intensity: float,
        tempo_dynamics: float,
        line_break_silence_seconds: float,
        pitch: float,
        volume_gain: float,
        api_key_override: str | None,
    ) -> tuple[bytes, str, str, int]:
        if not self.aivis_tts_endpoint:
            raise RuntimeError("AIVIS_TTS_ENDPOINT is not configured")
        headers = {}
        api_key = (api_key_override or "").strip() or self.aivis_api_key
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        payload = {
            **build_aivis_payload(
                voice_model=voice_model,
                voice_style=voice_style,
                text=text,
                speech_rate=speech_rate,
                emotional_intensity=emotional_intensity,
                tempo_dynamics=tempo_dynamics,
                line_break_silence_seconds=line_break_silence_seconds,
                pitch=pitch,
                volume_gain=volume_gain,
            )
        }
        rate_limit_key = api_key or "__default__"
        with httpx.Client(timeout=self.aivis_timeout_sec) as client:
            last_exc: Exception | None = None
            audio_bytes = b""
            for attempt in range(self.aivis_retry_attempts):
                AIVIS_RATE_LIMITER.acquire(rate_limit_key)
                try:
                    response = client.post(self.aivis_tts_endpoint, json=payload, headers=headers)
                    response.raise_for_status()
                    audio_bytes = response.content
                    break
                except httpx.HTTPStatusError as exc:
                    last_exc = exc
                    if exc.response is None or exc.response.status_code != 429 or attempt >= self.aivis_retry_attempts - 1:
                        raise
                    retry_after_sec = parse_retry_after_seconds(
                        exc.response.headers.get("Retry-After"),
                        self.aivis_retry_fallback_sec,
                    )
                    backoff_sec = self.aivis_retry_fallback_sec * (2**attempt)
                    time.sleep(max(retry_after_sec, backoff_sec))
            if not audio_bytes and last_exc is not None:
                raise last_exc
        duration_sec = probe_duration_seconds(audio_bytes, ".mp3")
        return audio_bytes, "audio/mpeg", ".mp3", duration_sec


def build_aivis_payload(
    *,
    voice_model: str,
    voice_style: str,
    text: str,
    speech_rate: float,
    emotional_intensity: float,
    tempo_dynamics: float,
    line_break_silence_seconds: float,
    pitch: float,
    volume_gain: float,
) -> dict:
    model_uuid = (voice_model or "").strip()
    speaker_uuid, style_id = parse_aivis_voice_style(voice_style)
    if not model_uuid:
        raise RuntimeError("Aivis model_uuid is empty")
    return {
        "model_uuid": model_uuid,
        "speaker_uuid": speaker_uuid,
        "style_id": style_id,
        "text": text,
        "use_ssml": True,
        "use_volume_normalizer": True,
        "speaking_rate": speech_rate if speech_rate > 0 else 1.0,
        "emotional_intensity": max(0.0, emotional_intensity),
        "tempo_dynamics": max(0.0, tempo_dynamics),
        "volume": max(0.0, 1.0 + volume_gain),
        "leading_silence_seconds": 0,
        "trailing_silence_seconds": 0.1,
        "line_break_silence_seconds": max(0.0, line_break_silence_seconds),
        "output_format": "mp3",
    }


def parse_aivis_voice_style(voice_style: str) -> tuple[str, int]:
    raw = (voice_style or "").strip()
    if not raw:
        raise RuntimeError("Aivis voice_style is empty; expected speaker_uuid:style_id")
    if raw.startswith("{"):
        data = json.loads(raw)
        speaker_uuid = str(data.get("speaker_uuid") or "").strip()
        style_id = int(data.get("style_id"))
        if not speaker_uuid:
            raise RuntimeError("Aivis speaker_uuid is empty")
        return speaker_uuid, style_id
    if ":" not in raw:
        raise RuntimeError("Aivis voice_style must be speaker_uuid:style_id")
    speaker_uuid, style_id_raw = raw.split(":", 1)
    speaker_uuid = speaker_uuid.strip()
    if not speaker_uuid:
        raise RuntimeError("Aivis speaker_uuid is empty")
    return speaker_uuid, int(style_id_raw.strip())


def synthesize_mock_audio(text: str, speech_rate: float) -> tuple[bytes, str, str, int]:
    speech_rate = speech_rate if speech_rate > 0 else 1.0
    duration_sec = max(1, int(math.ceil(max(len(text), 12) / 14 / speech_rate)))
    sample_rate = 24000
    total_frames = sample_rate * duration_sec
    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as wav_file:
        wav_file.setnchannels(1)
        wav_file.setsampwidth(2)
        wav_file.setframerate(sample_rate)
        wav_file.writeframes(b"\x00\x00" * total_frames)
    return buffer.getvalue(), "audio/wav", ".wav", duration_sec


def probe_duration_seconds(audio_bytes: bytes, suffix: str) -> int:
    if suffix == ".wav":
        with wave.open(io.BytesIO(audio_bytes), "rb") as wav_file:
            frames = wav_file.getnframes()
            rate = wav_file.getframerate()
            return max(1, int(math.ceil(frames / float(rate))))
    if suffix == ".mp3":
        info = MP3(io.BytesIO(audio_bytes)).info
        return max(1, int(math.ceil(float(info.length))))
    raise RuntimeError(f"unsupported audio format for duration probe: {suffix}")
