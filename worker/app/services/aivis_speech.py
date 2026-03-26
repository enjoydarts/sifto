import hashlib
import io
import math
import os
import threading
import time
import uuid
import wave

import httpx
from mutagen.mp3 import MP3

try:
    import redis
except Exception:  # pragma: no cover
    redis = None


class AivisRateLimiter:
    def __init__(self, min_interval_sec: float = 8.6) -> None:
        self.min_interval_sec = max(float(min_interval_sec or 0), 0.0)
        self._next_allowed_at: dict[str, float] = {}

    def acquire(self, key: str) -> None:
        if self.min_interval_sec <= 0:
            return
        normalized = (key or "__default__").strip() or "__default__"
        now = time.monotonic()
        next_allowed_at = self._next_allowed_at.get(normalized, 0.0)
        wait_sec = max(0.0, next_allowed_at - now)
        if wait_sec > 0:
            time.sleep(wait_sec)
            now += wait_sec
        self._next_allowed_at[normalized] = now + self.min_interval_sec


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


class AivisExecutionGate:
    def acquire(self, key: str) -> str | None:
        raise NotImplementedError

    def release(self, key: str, token: str | None) -> None:
        raise NotImplementedError


class AivisProcessExecutionGate(AivisExecutionGate):
    def __init__(self) -> None:
        self._locks: dict[str, threading.Lock] = {}
        self._guard = threading.Lock()

    def acquire(self, key: str) -> str | None:
        normalized = (key or "__default__").strip() or "__default__"
        with self._guard:
            lock = self._locks.get(normalized)
            if lock is None:
                lock = threading.Lock()
                self._locks[normalized] = lock
        lock.acquire()
        return normalized

    def release(self, key: str, token: str | None) -> None:
        normalized = (token or key or "__default__").strip() or "__default__"
        with self._guard:
            lock = self._locks.get(normalized)
        if lock is not None and lock.locked():
            lock.release()


class AivisRedisExecutionGate(AivisExecutionGate):
    def __init__(
        self,
        redis_client,
        lease_sec: float,
        fallback: AivisExecutionGate | None = None,
        poll_sec: float = 0.5,
    ) -> None:
        self.redis_client = redis_client
        self.lease_sec = max(float(lease_sec or 0), 1.0)
        self.fallback = fallback or AivisProcessExecutionGate()
        self.poll_sec = max(float(poll_sec or 0), 0.05)

    def acquire(self, key: str) -> str | None:
        if self.redis_client is None:
            return self.fallback.acquire(key)
        redis_key = self._redis_key(key)
        token = uuid.uuid4().hex
        lease_ms = max(int(math.ceil(self.lease_sec * 1000)), 1)
        while True:
            try:
                if self.redis_client.set(redis_key, token, nx=True, px=lease_ms):
                    return token
                remaining_ms = self.redis_client.pttl(redis_key)
            except Exception:
                return self.fallback.acquire(key)
            wait_sec = self.poll_sec if remaining_ms is None or remaining_ms < 0 else max(float(remaining_ms) / 1000.0, self.poll_sec)
            time.sleep(wait_sec)

    def release(self, key: str, token: str | None) -> None:
        if self.redis_client is None:
            self.fallback.release(key, token)
            return
        if not token:
            return
        redis_key = self._redis_key(key)
        try:
            current = self.redis_client.get(redis_key)
            if current == token:
                self.redis_client.delete(redis_key)
        except Exception:
            self.fallback.release(key, token)

    def _redis_key(self, key: str) -> str:
        raw = (key or "__default__").strip() or "__default__"
        digest = hashlib.sha256(raw.encode("utf-8")).hexdigest()
        return f"audio-briefing:aivis-synthesis-lock:{digest}"


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
_AIVIS_EXECUTION_LEASE_SEC = max(_env_float("AIVIS_TTS_TIMEOUT_SEC", 300.0) + 30.0, 30.0)
_AIVIS_FALLBACK_RATE_LIMITER = AivisRateLimiter(min_interval_sec=_AIVIS_MIN_INTERVAL_SEC)
AIVIS_PROCESS_EXECUTION_GATE = AivisProcessExecutionGate()
AIVIS_RATE_LIMITER = AivisRedisRateLimiter(
    redis_client=redis_client(),
    min_interval_sec=_AIVIS_MIN_INTERVAL_SEC,
    fallback=_AIVIS_FALLBACK_RATE_LIMITER,
    max_wait_sec=_AIVIS_REDIS_MAX_WAIT_SEC,
)
AIVIS_SYNTHESIS_GATE = AivisRedisExecutionGate(
    redis_client=redis_client(),
    lease_sec=_AIVIS_EXECUTION_LEASE_SEC,
    fallback=AIVIS_PROCESS_EXECUTION_GATE,
)

_AIVIS_RETRYABLE_STATUS_CODES = {429, 500, 502, 503, 504}


def parse_aivis_voice_style(voice_style: str) -> tuple[str, int]:
    raw = (voice_style or "").strip()
    if not raw:
        raise RuntimeError("Aivis voice_style is empty; expected speaker_uuid:style_id")
    if raw.startswith("{"):
        import json

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
    user_dictionary_uuid: str | None = None,
) -> dict:
    model_uuid = (voice_model or "").strip()
    speaker_uuid, style_id = parse_aivis_voice_style(voice_style)
    if not model_uuid:
        raise RuntimeError("Aivis model_uuid is empty")
    payload = {
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
    normalized_dictionary_uuid = (user_dictionary_uuid or "").strip()
    if normalized_dictionary_uuid:
        payload["user_dictionary_uuid"] = normalized_dictionary_uuid
    return payload


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


class AivisSpeechService:
    def __init__(self) -> None:
        self.aivis_tts_endpoint = os.getenv("AIVIS_TTS_ENDPOINT", "").strip()
        self.aivis_api_key = os.getenv("AIVIS_API_KEY", "").strip()
        self.aivis_retry_attempts = max(_env_int("AIVIS_TTS_RETRY_ATTEMPTS", 2), 1)
        self.aivis_retry_fallback_sec = max(_env_float("AIVIS_TTS_RETRY_FALLBACK_SEC", 9.0), 0.0)
        self.aivis_timeout_sec = max(_env_float("AIVIS_TTS_TIMEOUT_SEC", 300.0), 1.0)

    def synthesize(
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
        user_dictionary_uuid: str | None,
        api_key_override: str | None,
    ) -> tuple[bytes, str, str, int]:
        if not self.aivis_tts_endpoint:
            raise RuntimeError("AIVIS_TTS_ENDPOINT is not configured")
        headers = {}
        api_key = (api_key_override or "").strip() or self.aivis_api_key
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        payload = build_aivis_payload(
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
        )
        rate_limit_key = api_key or "__default__"
        with httpx.Client(timeout=self.aivis_timeout_sec) as client:
            last_exc: Exception | None = None
            audio_bytes = b""
            for attempt in range(self.aivis_retry_attempts):
                AIVIS_RATE_LIMITER.acquire(rate_limit_key)
                synthesis_token = AIVIS_SYNTHESIS_GATE.acquire(rate_limit_key)
                try:
                    try:
                        response = client.post(self.aivis_tts_endpoint, json=payload, headers=headers)
                        response.raise_for_status()
                        audio_bytes = response.content
                        break
                    except httpx.HTTPStatusError as exc:
                        last_exc = exc
                        status_code = exc.response.status_code if exc.response is not None else 0
                        if status_code not in _AIVIS_RETRYABLE_STATUS_CODES or attempt >= self.aivis_retry_attempts - 1:
                            raise
                        retry_after_sec = parse_retry_after_seconds(
                            exc.response.headers.get("Retry-After") if exc.response is not None else None,
                            self.aivis_retry_fallback_sec,
                        )
                        backoff_sec = self.aivis_retry_fallback_sec * (2**attempt)
                        time.sleep(max(retry_after_sec, backoff_sec))
                    except httpx.TimeoutException as exc:
                        last_exc = exc
                        if attempt >= self.aivis_retry_attempts - 1:
                            raise
                        backoff_sec = self.aivis_retry_fallback_sec * (2**attempt)
                        time.sleep(backoff_sec)
                finally:
                    AIVIS_SYNTHESIS_GATE.release(rate_limit_key, synthesis_token)
            if not audio_bytes and last_exc is not None:
                raise last_exc
        duration_sec = probe_duration_seconds(audio_bytes, ".mp3")
        return audio_bytes, "audio/mpeg", ".mp3", duration_sec
