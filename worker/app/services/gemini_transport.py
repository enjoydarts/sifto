import json
import os
import time
import hashlib
from datetime import datetime, timezone

import httpx
try:
    import redis
except Exception:  # pragma: no cover
    redis = None


_GEMINI_CONTEXT_CACHE: dict[str, tuple[str, float]] = {}
_GEMINI_CONTEXT_CACHE_SKIP: dict[str, float] = {}
_REDIS_CLIENT = None


def env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = float(raw)
        return v if v > 0 else default
    except Exception:
        return default


def env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = int(raw)
        return v if v > 0 else default
    except Exception:
        return default


def parse_rfc3339_utc(s: str) -> float | None:
    raw = (s or "").strip()
    if not raw:
        return None
    try:
        if raw.endswith("Z"):
            raw = raw[:-1] + "+00:00"
        return datetime.fromisoformat(raw).astimezone(timezone.utc).timestamp()
    except Exception:
        return None


def summary_context_cache_enabled() -> bool:
    return os.getenv("GEMINI_SUMMARY_CONTEXT_CACHE", "1").strip() not in ("0", "false", "False")


def audio_briefing_script_context_cache_enabled() -> bool:
    return os.getenv("GEMINI_AUDIO_BRIEFING_SCRIPT_CONTEXT_CACHE", "1").strip() not in ("0", "false", "False")


def context_cache_ttl_sec() -> int:
    raw = os.getenv("GEMINI_CONTEXT_CACHE_TTL_SEC")
    if raw is not None and raw != "":
        return env_int("GEMINI_CONTEXT_CACHE_TTL_SEC", 3600)
    return env_int("GEMINI_SUMMARY_CONTEXT_CACHE_TTL_SEC", 3600)


def cache_key_hash(parts: list[str]) -> str:
    h = hashlib.sha256()
    for p in parts:
        h.update((p or "").encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


def redis_client(logger):
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
    except Exception as e:
        logger.warning("gemini context cache redis init failed: %s", e)
        _REDIS_CLIENT = None
    return _REDIS_CLIENT


def redis_cache_get(cache_key: str, logger) -> tuple[str, float] | None:
    r = redis_client(logger)
    if r is None:
        return None
    try:
        raw = r.get(f"gemini:context-cache:{cache_key}")
    except Exception as e:
        logger.warning("gemini context cache redis get failed: %s", e)
        return None
    if not raw:
        return None
    try:
        data = json.loads(raw)
        name = str(data.get("name") or "").strip()
        exp = float(data.get("exp_ts", 0) or 0)
        if not name or exp <= 0:
            return None
        return name, exp
    except Exception:
        return None


def redis_cache_set(cache_key: str, name: str, exp_ts: float, ttl_sec: int, logger) -> None:
    r = redis_client(logger)
    if r is None:
        return
    payload = {"name": name, "exp_ts": exp_ts}
    try:
        r.setex(f"gemini:context-cache:{cache_key}", max(60, int(ttl_sec)), json.dumps(payload))
    except Exception as e:
        logger.warning("gemini context cache redis set failed: %s", e)


def is_cached_content_too_small_error(status_code: int, body_text: str) -> bool:
    if status_code != 400:
        return False
    s = (body_text or "").lower()
    return "cached content is too small" in s or "min_total_token_count" in s


def get_or_create_cached_content(model: str, api_key: str, cache_key: str, system_instruction: str, *, normalize_model_name, logger) -> str | None:
    now = time.time()
    skip_until = _GEMINI_CONTEXT_CACHE_SKIP.get(cache_key)
    if skip_until and skip_until > now:
        return None
    if skip_until and skip_until <= now:
        _GEMINI_CONTEXT_CACHE_SKIP.pop(cache_key, None)
    cached = _GEMINI_CONTEXT_CACHE.get(cache_key)
    if cached is not None:
        name, exp = cached
        if exp > now + 10:
            return name
        _GEMINI_CONTEXT_CACHE.pop(cache_key, None)
    redis_cached = redis_cache_get(cache_key, logger)
    if redis_cached is not None:
        name, exp = redis_cached
        if exp > now + 10:
            _GEMINI_CONTEXT_CACHE[cache_key] = (name, exp)
            return name

    ttl_sec = max(60, context_cache_ttl_sec())
    url = "https://generativelanguage.googleapis.com/v1beta/cachedContents"
    body = {
        "model": f"models/{normalize_model_name(model)}",
        "systemInstruction": {"parts": [{"text": system_instruction}]},
        "ttl": f"{ttl_sec}s",
    }
    req_timeout = env_timeout_seconds("GEMINI_TIMEOUT_SEC", 90.0)
    with httpx.Client(timeout=req_timeout) as client:
        resp = client.post(url, json=body, params={"key": api_key})
    if resp.status_code >= 400:
        if is_cached_content_too_small_error(resp.status_code, resp.text):
            _GEMINI_CONTEXT_CACHE_SKIP[cache_key] = now + 1800
            logger.info("gemini context cache skipped (too small) key=%s", cache_key[:16])
            return None
        raise RuntimeError(f"gemini cachedContents create failed status={resp.status_code} body={resp.text[:1000]}")
    data = resp.json() if resp.content else {}
    name = str(data.get("name") or "").strip()
    if not name:
        return None
    expire_ts = parse_rfc3339_utc(str(data.get("expireTime") or "")) or (now + ttl_sec)
    _GEMINI_CONTEXT_CACHE[cache_key] = (name, expire_ts)
    redis_cache_set(cache_key, name, expire_ts, ttl_sec, logger)
    return name


def normalize_response_schema(value):
    if isinstance(value, dict):
        normalized = {}
        for key, item in value.items():
            if key == "additionalProperties":
                continue
            normalized[key] = normalize_response_schema(item)
        return normalized
    if isinstance(value, list):
        return [normalize_response_schema(item) for item in value]
    return value


def generate_content(
    prompt: str,
    model: str,
    api_key: str,
    *,
    normalize_model_name,
    logger,
    max_output_tokens: int = 1024,
    response_schema: dict | None = None,
    timeout_sec: float | None = None,
    system_instruction: str | None = None,
    context_cache_key: str | None = None,
    response_mime_type: str = "application/json",
) -> tuple[str, dict]:
    if not api_key:
        raise RuntimeError("google api key is required")
    model_name = normalize_model_name(model)
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{model_name}:generateContent"
    generation_config: dict = {
        "temperature": 0.2,
        "maxOutputTokens": max_output_tokens,
        "responseMimeType": response_mime_type,
    }
    if response_schema:
        generation_config["responseSchema"] = normalize_response_schema(response_schema)

    body = {"contents": [{"role": "user", "parts": [{"text": prompt}]}], "generationConfig": generation_config}
    cached_content_name = ""
    if system_instruction:
        body["systemInstruction"] = {"parts": [{"text": system_instruction}]}
        if context_cache_key:
            try:
                cached_name = get_or_create_cached_content(
                    model_name,
                    api_key,
                    context_cache_key,
                    system_instruction,
                    normalize_model_name=normalize_model_name,
                    logger=logger,
                )
                if cached_name:
                    body.pop("systemInstruction", None)
                    body["cachedContent"] = cached_name
                    cached_content_name = cached_name
            except Exception as e:
                logger.warning("gemini context cache unavailable key=%s err=%s", context_cache_key[:16], e)
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else env_timeout_seconds("GEMINI_TIMEOUT_SEC", 90.0)
    attempts = env_int("GEMINI_RETRY_ATTEMPTS", 3)
    base_sleep_sec = env_timeout_seconds("GEMINI_RETRY_BASE_SEC", 0.5)
    retryable_status = {408, 409, 429, 500, 502, 503, 504}
    resp: httpx.Response | None = None
    last_error: Exception | None = None
    for i in range(attempts):
        try:
            with httpx.Client(timeout=req_timeout) as client:
                resp = client.post(url, json=body, params={"key": api_key})
        except Exception as e:
            last_error = e
            if i < attempts - 1:
                time.sleep(base_sleep_sec * (2**i))
                continue
            raise RuntimeError(f"gemini generateContent request failed: {e}") from e
        if resp.status_code < 400:
            break
        if cached_content_name and resp.status_code in (400, 404):
            body.pop("cachedContent", None)
            if system_instruction:
                body["systemInstruction"] = {"parts": [{"text": system_instruction}]}
            cached_content_name = ""
            if i < attempts - 1:
                continue
        if resp.status_code in retryable_status and i < attempts - 1:
            time.sleep(base_sleep_sec * (2**i))
            continue
        raise RuntimeError(f"gemini generateContent failed status={resp.status_code} body={resp.text[:1000]}")

    if resp is None:
        if last_error:
            raise RuntimeError(f"gemini generateContent request failed: {last_error}") from last_error
        raise RuntimeError("gemini generateContent failed: no response")
    data = resp.json()
    usage_meta = data.get("usageMetadata", {}) if isinstance(data, dict) else {}
    prompt_token_count = int(usage_meta.get("promptTokenCount", 0) or 0)
    cached_content_token_count = int(usage_meta.get("cachedContentTokenCount", 0) or 0)
    tool_use_prompt_token_count = int(usage_meta.get("toolUsePromptTokenCount", 0) or 0)
    thoughts_token_count = int(usage_meta.get("thoughtsTokenCount", 0) or 0)
    candidates_token_count = int(usage_meta.get("candidatesTokenCount", 0) or 0)
    usage = {
        "input_tokens": prompt_token_count + tool_use_prompt_token_count,
        "output_tokens": candidates_token_count + thoughts_token_count,
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": cached_content_token_count,
        "prompt_tokens_raw": prompt_token_count,
    }
    candidates = data.get("candidates", []) if isinstance(data, dict) else []
    if not candidates:
        return "", usage
    parts = candidates[0].get("content", {}).get("parts", [])
    text = ""
    for p in parts:
        t = p.get("text")
        if isinstance(t, str):
            text += t
    return text.strip(), usage
