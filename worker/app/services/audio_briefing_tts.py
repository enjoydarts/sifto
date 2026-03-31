import base64
import io
import math
import os
import threading
import wave

import boto3
import httpx
from app.services.aivis_speech import AIVIS_RATE_LIMITER, AivisRateLimiter, AivisRedisRateLimiter, AivisSpeechService, build_aivis_payload


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


class AudioBriefingHeartbeatLoop:
    def __init__(self, heartbeat_url: str | None, heartbeat_token: str | None, interval_sec: float, timeout_sec: float) -> None:
        self.heartbeat_url = (heartbeat_url or "").strip()
        self.heartbeat_token = (heartbeat_token or "").strip()
        self.interval_sec = max(float(interval_sec or 0), 1.0)
        self.timeout_sec = max(float(timeout_sec or 0), 1.0)
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None

    def start(self) -> None:
        if not self.enabled():
            return
        self._send_once()
        self._thread = threading.Thread(target=self._run, name="audio-briefing-heartbeat", daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        if self._thread is not None:
            self._thread.join(timeout=max(self.timeout_sec, 1.0))

    def enabled(self) -> bool:
        return bool(self.heartbeat_url and self.heartbeat_token)

    def _run(self) -> None:
        while not self._stop.wait(self.interval_sec):
            self._send_once()

    def _send_once(self) -> None:
        if not self.enabled():
            return
        try:
            with httpx.Client(timeout=self.timeout_sec) as client:
                response = client.post(
                    self.heartbeat_url,
                    headers={"Authorization": f"Bearer {self.heartbeat_token}"},
                )
                response.raise_for_status()
        except Exception:
            return


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
        self.heartbeat_interval_sec = max(_env_float("AUDIO_BRIEFING_HEARTBEAT_INTERVAL_SEC", 20.0), 1.0)
        self.heartbeat_timeout_sec = max(_env_float("AUDIO_BRIEFING_HEARTBEAT_TIMEOUT_SEC", 10.0), 1.0)
        self.aivis = AivisSpeechService()

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
        chunk_trailing_silence_seconds: float,
        pitch: float,
        volume_gain: float,
        output_object_key: str,
        chunk_id: str | None = None,
        heartbeat_url: str | None = None,
        heartbeat_token: str | None = None,
        user_dictionary_uuid: str | None = None,
        aivis_api_key: str | None = None,
    ) -> tuple[str, int]:
        _ = (chunk_id or "").strip()
        heartbeat = AudioBriefingHeartbeatLoop(
            heartbeat_url=heartbeat_url,
            heartbeat_token=heartbeat_token,
            interval_sec=self.heartbeat_interval_sec,
            timeout_sec=self.heartbeat_timeout_sec,
        )
        heartbeat.start()
        try:
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
                    chunk_trailing_silence_seconds=chunk_trailing_silence_seconds,
                    pitch=pitch,
                    volume_gain=volume_gain,
                    user_dictionary_uuid=user_dictionary_uuid,
                    api_key_override=aivis_api_key,
                )
            else:
                raise RuntimeError(f"unsupported tts provider: {provider}")

            if not output_object_key.endswith(suffix):
                output_object_key = output_object_key + suffix
            self.upload_bytes(output_object_key, payload, content_type)
            return output_object_key, duration_sec
        finally:
            heartbeat.stop()

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

    def stat_object(self, object_key: str, bucket_override: str | None = None) -> int:
        client = self.r2_client()
        result = client.head_object(
            Bucket=self.resolve_bucket(bucket_override),
            Key=object_key,
        )
        size = int(result.get("ContentLength") or 0)
        if size < 0:
            size = 0
        return size

    def upload_base64_object(self, object_key: str, content_base64: str, content_type: str, bucket_override: str | None = None) -> str:
        payload = base64.b64decode((content_base64 or "").encode("utf-8"), validate=True)
        self.upload_bytes(object_key, payload, content_type, bucket_override=bucket_override)
        return object_key

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
        chunk_trailing_silence_seconds: float,
        pitch: float,
        volume_gain: float,
        user_dictionary_uuid: str | None,
        api_key_override: str | None,
    ) -> tuple[bytes, str, str, int]:
        self.aivis.aivis_tts_endpoint = self.aivis_tts_endpoint
        self.aivis.aivis_api_key = self.aivis_api_key
        self.aivis.aivis_retry_attempts = self.aivis_retry_attempts
        self.aivis.aivis_retry_fallback_sec = self.aivis_retry_fallback_sec
        self.aivis.aivis_timeout_sec = self.aivis_timeout_sec
        return self.aivis.synthesize(
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
            api_key_override=api_key_override,
        )


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
