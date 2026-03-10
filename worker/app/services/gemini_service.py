import json
import logging
import os
import re
import time
import hashlib
from urllib.parse import urlparse
from datetime import datetime, timezone

import httpx
try:
    import redis
except Exception:  # pragma: no cover
    redis = None
from app.services.llm_catalog import model_pricing
from app.services.summary_faithfulness_common import (
    SUMMARY_FAITHFULNESS_SCHEMA,
    normalize_summary_faithfulness_result,
    summary_faithfulness_prompt,
    summary_faithfulness_system_instruction,
)

_log = logging.getLogger(__name__)
_GEMINI_PRICING_SOURCE_VERSION = "google_aistudio_static_2026_02"
_GEMINI_CONTEXT_CACHE: dict[str, tuple[str, float]] = {}
_GEMINI_CONTEXT_CACHE_SKIP: dict[str, float] = {}
_REDIS_CLIENT = None

_LEGACY_MODEL_PRICING = {
    "gemini-3-flash-preview": {"input_per_mtok_usd": 0.5, "output_per_mtok_usd": 3.0, "cache_read_per_mtok_usd": 0.05},
    "gemini-3.1-flash-lite-preview": {"input_per_mtok_usd": 0.25, "output_per_mtok_usd": 1.5, "cache_read_per_mtok_usd": 0.025},
    # Alias kept for forward compatibility if/when preview suffix is removed.
    "gemini-3.1-flash-lite": {"input_per_mtok_usd": 0.25, "output_per_mtok_usd": 1.5, "cache_read_per_mtok_usd": 0.025},
    # <=200k prompt tokens. >200k tier is handled in _estimate_cost_usd.
    "gemini-3.1-pro-preview": {"input_per_mtok_usd": 2.0, "output_per_mtok_usd": 12.0, "cache_read_per_mtok_usd": 0.20},
    # Deprecated model kept for backward-compat pricing on existing user settings.
    "gemini-3-pro-preview": {"input_per_mtok_usd": 2.0, "output_per_mtok_usd": 12.0, "cache_read_per_mtok_usd": 0.20},
    # USD per 1M tokens (input/output).
    "gemini-2.5-flash": {"input_per_mtok_usd": 0.3, "output_per_mtok_usd": 2.5, "cache_read_per_mtok_usd": 0.03},
    "gemini-2.5-flash-lite": {"input_per_mtok_usd": 0.1, "output_per_mtok_usd": 0.4, "cache_read_per_mtok_usd": 0.01},
    "gemini-2.5-pro": {"input_per_mtok_usd": 1.25, "output_per_mtok_usd": 10.0, "cache_read_per_mtok_usd": 0.125},
    # Legacy/deprecated families kept for backward compatibility in historical logs/user settings.
    "gemini-2.0-flash": {"input_per_mtok_usd": 0.1, "output_per_mtok_usd": 0.4, "cache_read_per_mtok_usd": 0.0},
    "gemini-2.0-flash-lite": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.3, "cache_read_per_mtok_usd": 0.0},
    "gemini-1.5-flash": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.3, "cache_read_per_mtok_usd": 0.0},
    "gemini-1.5-pro": {"input_per_mtok_usd": 1.25, "output_per_mtok_usd": 5.0, "cache_read_per_mtok_usd": 0.0},
}


def _env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = float(raw)
        return v if v > 0 else default
    except Exception:
        return default


def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = int(raw)
        return v if v > 0 else default
    except Exception:
        return default


def _clamp01(v, default: float = 0.5) -> float:
    try:
        x = float(v)
    except Exception:
        return default
    if x < 0:
        return 0.0
    if x > 1:
        return 1.0
    return x


def _summary_composite_score(breakdown: dict) -> float:
    weights = {
        "importance": 0.38,
        "novelty": 0.22,
        "actionability": 0.18,
        "reliability": 0.17,
        "relevance": 0.05,
    }
    total = 0.0
    for k, w in weights.items():
        total += _clamp01(breakdown.get(k, 0.5), 0.5) * w
    return round(total, 4)


def _clamp_int(v: int, lo: int, hi: int) -> int:
    return max(lo, min(hi, int(v)))


def _target_summary_chars(source_text_chars: int | None, facts: list[str]) -> int:
    if isinstance(source_text_chars, int) and source_text_chars > 0:
        return _clamp_int(round(source_text_chars * 0.16), 220, 1200)
    facts_chars = sum(len(str(f)) for f in (facts or []))
    if facts_chars > 0:
        return _clamp_int(round(facts_chars * 0.9), 220, 900)
    return 300


def _summary_max_tokens(target_chars: int) -> int:
    return _clamp_int(round(target_chars * 1.2), 700, 2600)


def _digest_primary_topic(item: dict) -> str:
    topics = item.get("topics") or []
    if isinstance(topics, list):
        for t in topics:
            s = str(t).strip()
            if s:
                return s[:40]
    return "その他"


def _digest_item_score(item: dict) -> float:
    try:
        return float(item.get("score", 0.0) or 0.0)
    except Exception:
        return 0.0


def _build_digest_input_sections(items: list[dict]) -> tuple[str, str]:
    if len(items) <= 80:
        summary_limit = 450 if len(items) <= 20 else 240 if len(items) <= 50 else 120
        lines = []
        for idx, item in enumerate(items, start=1):
            rank = item.get("rank")
            title = item.get("title") or "（タイトルなし）"
            summary = str(item.get("summary") or "")[:summary_limit]
            topics = ", ".join(item.get("topics") or [])
            score = item.get("score")
            lines.append(
                f"- item={idx} rank={rank} | title={title} | topics={topics} | score={score} | summary={summary}"
            )
        return "items", "\n".join(lines)

    sorted_items = sorted(
        items,
        key=lambda x: (
            int(x.get("rank") or 10**9),
            -_digest_item_score(x),
        ),
    )
    highlights = sorted_items[: min(24, len(sorted_items))]

    groups: dict[str, list[dict]] = {}
    for item in items:
        groups.setdefault(_digest_primary_topic(item), []).append(item)

    ordered_groups = sorted(
        groups.items(),
        key=lambda kv: (-len(kv[1]), -max((_digest_item_score(i) for i in kv[1]), default=0.0), kv[0]),
    )

    lines: list[str] = []
    lines.append("[top_items]")
    for idx, item in enumerate(highlights, start=1):
        title = item.get("title") or "（タイトルなし）"
        summary = str(item.get("summary") or "")[:140]
        topics = ", ".join(item.get("topics") or [])
        rank = item.get("rank")
        score = item.get("score")
        lines.append(
            f"- top={idx} rank={rank} | title={title} | topics={topics} | score={score} | summary={summary}"
        )

    lines.append("")
    lines.append("[topic_groups]")
    for topic, topic_items in ordered_groups[:40]:
        sorted_topic_items = sorted(
            topic_items,
            key=lambda x: (
                int(x.get("rank") or 10**9),
                -_digest_item_score(x),
            ),
        )
        sample_titles = [str(i.get("title") or "（タイトルなし）")[:60] for i in sorted_topic_items[:4]]
        sample_summaries = [str(i.get("summary") or "")[:90] for i in sorted_topic_items[:3]]
        avg_score = round(
            sum(_digest_item_score(i) for i in topic_items) / max(1, len(topic_items)),
            3,
        )
        lines.append(
            f"- topic={topic} | count={len(topic_items)} | avg_score={avg_score} | "
            f"sample_titles={' / '.join(sample_titles)} | sample_summaries={' / '.join(sample_summaries)}"
        )

    return "topic_grouped", "\n".join(lines)


def _normalize_model_name(model: str) -> str:
    m = str(model or "").strip()
    if m.startswith("models/"):
        return m[7:]
    if "/models/" in m:
        return m.split("/models/", 1)[1]
    return m


def _normalize_model_family(model: str) -> str:
    m = _normalize_model_name(model)
    if model_pricing(m) is not None:
        return m
    for family in sorted(_LEGACY_MODEL_PRICING.keys(), key=len, reverse=True):
        if m == family or m.startswith(family + "-"):
            return family
    return m


def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _pricing_for_model(model: str, purpose: str) -> dict:
    family = _normalize_model_family(model)
    base = dict(
        model_pricing(family)
        or model_pricing(model)
        or _LEGACY_MODEL_PRICING.get(
            family,
            {
                "input_per_mtok_usd": 0.0,
                "output_per_mtok_usd": 0.0,
                "cache_read_per_mtok_usd": 0.0,
            },
        )
    )
    source = str(base.get("pricing_source") or _GEMINI_PRICING_SOURCE_VERSION)
    prefix = f"GEMINI_{purpose.upper()}_"
    override_map = {
        "input_per_mtok_usd": _env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
        "output_per_mtok_usd": _env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
        "cache_read_per_mtok_usd": _env_optional_float(prefix + "CACHE_READ_PER_MTOK_USD"),
    }
    for k, v in override_map.items():
        if v is not None:
            base[k] = v
            source = "env_override"
    base["pricing_source"] = source
    base["pricing_model_family"] = family
    return base


def _estimate_cost_usd(model: str, purpose: str, usage: dict) -> float:
    p = _pricing_for_model(model, purpose)
    family = _normalize_model_family(model)
    input_rate = p["input_per_mtok_usd"]
    output_rate = p["output_per_mtok_usd"]
    cache_read_rate = p.get("cache_read_per_mtok_usd", 0.0)
    # Gemini Pro preview families have two pricing tiers by prompt length.
    prompt_size_for_tier = usage.get("prompt_tokens_raw", usage.get("input_tokens", 0))
    if family in ("gemini-3.1-pro-preview", "gemini-3-pro-preview") and prompt_size_for_tier > 200_000:
        input_rate = 4.0
        output_rate = 18.0
        cache_read_rate = 0.40
    if family == "gemini-2.5-pro" and prompt_size_for_tier > 200_000:
        input_rate = 2.5
        output_rate = 15.0
        cache_read_rate = 0.25
    non_cached_input_tokens = max(0, int(usage.get("input_tokens", 0) or 0) - int(usage.get("cache_read_input_tokens", 0) or 0))
    total = 0.0
    total += non_cached_input_tokens / 1_000_000 * input_rate
    total += usage.get("output_tokens", 0) / 1_000_000 * output_rate
    total += usage.get("cache_read_input_tokens", 0) / 1_000_000 * cache_read_rate
    return round(total, 8)


def _llm_meta(model: str, purpose: str, usage: dict) -> dict:
    pricing = _pricing_for_model(model, purpose)
    actual_model = _normalize_model_name(model)
    return {
        "provider": "google",
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", _GEMINI_PRICING_SOURCE_VERSION),
        "input_tokens": usage.get("input_tokens", 0),
        "output_tokens": usage.get("output_tokens", 0),
        "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
        "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }


def _parse_rfc3339_utc(s: str) -> float | None:
    raw = (s or "").strip()
    if not raw:
        return None
    try:
        if raw.endswith("Z"):
            raw = raw[:-1] + "+00:00"
        return datetime.fromisoformat(raw).astimezone(timezone.utc).timestamp()
    except Exception:
        return None


def _summary_context_cache_enabled() -> bool:
    return os.getenv("GEMINI_SUMMARY_CONTEXT_CACHE", "1").strip() not in ("0", "false", "False")


def _summary_context_cache_ttl_sec() -> int:
    return _env_int("GEMINI_SUMMARY_CONTEXT_CACHE_TTL_SEC", 3600)


def _cache_key_hash(parts: list[str]) -> str:
    h = hashlib.sha256()
    for p in parts:
        h.update((p or "").encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


def _redis_client():
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
        _log.warning("gemini context cache redis init failed: %s", e)
        _REDIS_CLIENT = None
    return _REDIS_CLIENT


def _redis_cache_get(cache_key: str) -> tuple[str, float] | None:
    r = _redis_client()
    if r is None:
        return None
    try:
        raw = r.get(f"gemini:context-cache:{cache_key}")
    except Exception as e:
        _log.warning("gemini context cache redis get failed: %s", e)
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


def _redis_cache_set(cache_key: str, name: str, exp_ts: float, ttl_sec: int) -> None:
    r = _redis_client()
    if r is None:
        return
    payload = {"name": name, "exp_ts": exp_ts}
    try:
        r.setex(f"gemini:context-cache:{cache_key}", max(60, int(ttl_sec)), json.dumps(payload))
    except Exception as e:
        _log.warning("gemini context cache redis set failed: %s", e)


def _is_cached_content_too_small_error(status_code: int, body_text: str) -> bool:
    if status_code != 400:
        return False
    s = (body_text or "").lower()
    return (
        "cached content is too small" in s
        or "min_total_token_count" in s
    )


def _get_or_create_cached_content(model: str, api_key: str, cache_key: str, system_instruction: str) -> str | None:
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
    redis_cached = _redis_cache_get(cache_key)
    if redis_cached is not None:
        name, exp = redis_cached
        if exp > now + 10:
            _GEMINI_CONTEXT_CACHE[cache_key] = (name, exp)
            return name

    ttl_sec = max(60, _summary_context_cache_ttl_sec())
    url = "https://generativelanguage.googleapis.com/v1beta/cachedContents"
    body = {
        "model": f"models/{_normalize_model_name(model)}",
        "systemInstruction": {"parts": [{"text": system_instruction}]},
        "ttl": f"{ttl_sec}s",
    }
    req_timeout = _env_timeout_seconds("GEMINI_TIMEOUT_SEC", 90.0)
    with httpx.Client(timeout=req_timeout) as client:
        resp = client.post(url, json=body, params={"key": api_key})
    if resp.status_code >= 400:
        if _is_cached_content_too_small_error(resp.status_code, resp.text):
            # Do not fail summary generation; skip cache create for this key for a while.
            _GEMINI_CONTEXT_CACHE_SKIP[cache_key] = now + 1800
            _log.info("gemini context cache skipped (too small) key=%s", cache_key[:16])
            return None
        raise RuntimeError(f"gemini cachedContents create failed status={resp.status_code} body={resp.text[:1000]}")
    data = resp.json() if resp.content else {}
    name = str(data.get("name") or "").strip()
    if not name:
        return None
    expire_ts = _parse_rfc3339_utc(str(data.get("expireTime") or "")) or (now + ttl_sec)
    _GEMINI_CONTEXT_CACHE[cache_key] = (name, expire_ts)
    _redis_cache_set(cache_key, name, expire_ts, ttl_sec)
    return name


def _generate_content(
    prompt: str,
    model: str,
    api_key: str,
    max_output_tokens: int = 1024,
    response_schema: dict | None = None,
    timeout_sec: float | None = None,
    system_instruction: str | None = None,
    context_cache_key: str | None = None,
) -> tuple[str, dict]:
    if not api_key:
        raise RuntimeError("google api key is required")
    model_name = _normalize_model_name(model)
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{model_name}:generateContent"
    generation_config: dict = {
        "temperature": 0.2,
        "maxOutputTokens": max_output_tokens,
        "responseMimeType": "application/json",
    }
    if response_schema:
        generation_config["responseSchema"] = response_schema

    body = {"contents": [{"role": "user", "parts": [{"text": prompt}]}], "generationConfig": generation_config}
    cached_content_name = ""
    if system_instruction:
        body["systemInstruction"] = {"parts": [{"text": system_instruction}]}
        if context_cache_key and _summary_context_cache_enabled():
            try:
                cached_name = _get_or_create_cached_content(model_name, api_key, context_cache_key, system_instruction)
                if cached_name:
                    body.pop("systemInstruction", None)
                    body["cachedContent"] = cached_name
                    cached_content_name = cached_name
            except Exception as e:
                _log.warning("gemini context cache unavailable key=%s err=%s", context_cache_key[:16], e)
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("GEMINI_TIMEOUT_SEC", 90.0)
    attempts = _env_int("GEMINI_RETRY_ATTEMPTS", 3)
    base_sleep_sec = _env_timeout_seconds("GEMINI_RETRY_BASE_SEC", 0.5)
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
            # Fallback when cachedContent is expired/invalid server-side.
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
    # Google pricing: output pricing includes thinking tokens.
    output_tokens_billed = candidates_token_count + thoughts_token_count
    # promptTokenCount includes cached content when cachedContent is used.
    # Keep raw prompt for tier selection and add tool-use prompt tokens as billable input.
    input_tokens_billed = prompt_token_count + tool_use_prompt_token_count
    usage = {
        "input_tokens": input_tokens_billed,
        "output_tokens": output_tokens_billed,
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


def _parse_json_string_array(text: str) -> list[str]:
    start = text.find("[")
    end = text.rfind("]") + 1
    if start == -1 or end == 0:
        return []
    try:
        data = json.loads(text[start:end])
    except Exception:
        return []
    return [str(v) for v in data if isinstance(v, str)]


def _strip_code_fence(text: str) -> str:
    s = (text or "").strip().lstrip("\ufeff")
    if s.startswith("```"):
        s = re.sub(r"^```[a-zA-Z0-9_-]*\n?", "", s)
        s = re.sub(r"\n?```$", "", s).strip()
    return s


def _extract_first_json_object(text: str) -> dict | None:
    s = _strip_code_fence(text)
    if not s:
        return None
    decoder = json.JSONDecoder()
    idx = s.find("{")
    while idx >= 0:
        try:
            obj, _ = decoder.raw_decode(s[idx:])
            if isinstance(obj, dict):
                return obj
        except Exception:
            pass
        idx = s.find("{", idx + 1)
    return None


def _normalize_url_for_match(raw: str) -> str:
    s = (raw or "").strip()
    if not s:
        return ""
    try:
        u = urlparse(s)
    except Exception:
        return s.lower()
    scheme = (u.scheme or "https").lower()
    host = (u.netloc or "").lower()
    path = (u.path or "").rstrip("/")
    return f"{scheme}://{host}{path}"


def _decode_json_string_fragment(raw: str) -> str:
    try:
        return json.loads(f'"{raw}"')
    except Exception:
        return raw.replace("\\n", "\n").replace('\\"', '"').replace("\\\\", "\\")


def _extract_json_string_value_loose(text: str, field: str) -> str:
    s = _strip_code_fence(text)
    key = f'"{field}"'
    i = s.find(key)
    if i < 0:
        return ""
    rest = s[i + len(key):]
    colon = rest.find(":")
    if colon < 0:
        return ""
    raw = rest[colon + 1 :].lstrip()
    if not raw.startswith('"'):
        return ""
    raw = raw[1:]
    out: list[str] = []
    escaped = False
    for ch in raw:
        if escaped:
            out.append(ch)
            escaped = False
            continue
        if ch == "\\":
            out.append(ch)
            escaped = True
            continue
        if ch == '"':
            break
        out.append(ch)
    return _decode_json_string_fragment("".join(out)).strip()


def _extract_compose_digest_fields(text: str) -> tuple[str, str]:
    data = _extract_first_json_object(text) or {}
    subject = str(data.get("subject") or "").strip()
    body = str(data.get("body") or "").strip()
    if subject and body:
        return subject, body

    s = _strip_code_fence(text)
    m_subject = re.search(r'"subject"\s*:\s*"((?:\\.|[^"\\])*)"', s, re.S)
    if not subject and m_subject:
        subject = _decode_json_string_fragment(m_subject.group(1)).strip()

    m_body = re.search(r'"body"\s*:\s*"((?:\\.|[^"\\])*)"', s, re.S)
    if not body and m_body:
        body = _decode_json_string_fragment(m_body.group(1)).strip()
    elif not body:
        key = '"body"'
        i = s.find(key)
        if i >= 0:
            rest = s[i + len(key):]
            colon = rest.find(":")
            if colon >= 0:
                raw = rest[colon + 1 :].strip()
                if raw.startswith('"'):
                    raw = raw[1:]
                marker_idx = raw.find('",\n  "sections"')
                if marker_idx < 0:
                    marker_idx = raw.find('", "sections"')
                if marker_idx > 0:
                    raw = raw[:marker_idx]
                raw = raw.strip().rstrip('"').strip()
                if raw:
                    body = raw.replace("\\n", "\n").replace('\\"', '"').strip()
    return subject, body


def _contains_japanese(text: str) -> bool:
    s = (text or "").strip()
    if not s:
        return False
    return re.search(r"[\u3040-\u30ff\u3400-\u9fff]", s) is not None


def _needs_title_translation(title: str | None, translated_title: str) -> bool:
    src = (title or "").strip()
    if not src:
        return False
    if (translated_title or "").strip():
        return False
    if _contains_japanese(src):
        return False
    return re.search(r"[A-Za-z]", src) is not None


def _translate_title_to_ja(title: str, model: str, api_key: str) -> str:
    prompt = f"""次の英語タイトルを自然な日本語に翻訳してください。
JSONで返してください:
{{
  "translated_title": "日本語タイトル"
}}

タイトル: {title}
"""
    schema = {
        "type": "object",
        "properties": {
            "translated_title": {"type": "string"},
        },
        "required": ["translated_title"],
    }
    text, _ = _generate_content(
        prompt,
        model=model,
        api_key=api_key,
        max_output_tokens=200,
        response_schema=schema,
    )
    data = _extract_first_json_object(text) or {}
    candidate = str(data.get("translated_title") or "").strip()
    if not candidate:
        candidate = _strip_code_fence(text).strip().strip('"').strip("'")
    if not candidate:
        plain_prompt = f"""次のタイトルを日本語に翻訳してください。
説明は不要です。翻訳結果のみを1行で返してください。

タイトル: {title}
"""
        plain_text, _ = _generate_content(
            plain_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=120,
            response_schema=None,
        )
        candidate = _strip_code_fence(plain_text).strip().strip('"').strip("'")
    return candidate[:300]


def rank_feed_suggestions(
    existing_sources: list[dict],
    preferred_topics: list[str],
    candidates: list[dict],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
    model: str,
    api_key: str,
) -> dict:
    existing_sources = existing_sources[:40]
    preferred_topics = [str(t).strip() for t in preferred_topics if str(t).strip()][:20]
    candidates = candidates[:80]
    positive_examples = (positive_examples or [])[:8]
    negative_examples = (negative_examples or [])[:5]
    prompt = f"""あなたはRSSフィードの推薦アシスタントです。
既存の購読ソース・興味トピック・候補フィードを見て、ユーザーに合いそうな候補を順位付けしてください。

要件:
- 候補は必ず id で指定する（urlは補助情報で、新規URLを作らない）
- 既存ソースと重複しすぎる候補は下げる
- 興味トピックに近い候補を優先
- 理由は日本語で短く（40〜100字）
- JSONのみで返す

返却形式:
{{
  "items": [
    {{"id":"c001", "reason":"...", "confidence":0.0-1.0}}
  ]
}}

Few-shot（好みの既存Feed例）:
{json.dumps(positive_examples, ensure_ascii=False)}

Few-shot（避けたい傾向の既存Feed例）:
{json.dumps(negative_examples, ensure_ascii=False)}

既存ソース:
{json.dumps(existing_sources, ensure_ascii=False)}

興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}

候補フィード:
{json.dumps(candidates, ensure_ascii=False)}
"""
    rank_schema = {
        "type": "OBJECT",
        "properties": {
            "items": {
                "type": "ARRAY",
                "items": {
                    "type": "OBJECT",
                    "properties": {
                        "id": {"type": "STRING"},
                        "reason": {"type": "STRING"},
                        "confidence": {"type": "NUMBER"},
                    },
                    "required": ["id", "reason", "confidence"],
                },
            }
        },
        "required": ["items"],
    }
    text, usage = _generate_content(
        prompt,
        model=model,
        api_key=api_key,
        max_output_tokens=2800,
        response_schema=rank_schema,
    )
    data = _extract_first_json_object(text) or {}
    rows = data.get("items", [])
    if not isinstance(rows, list):
        rows = []
    allowed_ids = {str(c.get("id") or "").strip() for c in candidates if str(c.get("id") or "").strip()}
    out: list[dict] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        cid = str(row.get("id") or "").strip()
        if not cid or cid not in allowed_ids:
            continue
        reason = str(row.get("reason") or "").strip()[:180]
        try:
            confidence = _clamp01(float(row.get("confidence", 0.5)), 0.5)
        except Exception:
            confidence = 0.5
        out.append({"id": cid, "url": "", "reason": reason, "confidence": confidence})
    # Rescue: Gemini が空配列を返した場合は簡易再プロンプトで再取得する。
    if len(out) == 0 and len(candidates) > 0:
        rescue_prompt = f"""候補フィードを優先度順に再提示してください。必ず最低10件は返してください。
JSONのみ:
{{
  "items":[{{"id":"c001","reason":"短い理由","confidence":0.0-1.0}}]
}}

興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}

候補フィード:
{json.dumps(candidates, ensure_ascii=False)}
"""
        rescue_text, rescue_usage = _generate_content(
            rescue_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=1800,
            response_schema=rank_schema,
        )
        rescue_data = _extract_first_json_object(rescue_text) or {}
        rescue_rows = rescue_data.get("items", [])
        if isinstance(rescue_rows, list):
            for row in rescue_rows:
                if not isinstance(row, dict):
                    continue
                cid = str(row.get("id") or "").strip()
                if not cid or cid not in allowed_ids:
                    continue
                reason = str(row.get("reason") or "").strip()[:180]
                try:
                    confidence = _clamp01(float(row.get("confidence", 0.5)), 0.5)
                except Exception:
                    confidence = 0.5
                out.append({"id": cid, "url": "", "reason": reason, "confidence": confidence})
        usage["input_tokens"] = int(usage.get("input_tokens", 0)) + int(rescue_usage.get("input_tokens", 0))
        usage["output_tokens"] = int(usage.get("output_tokens", 0)) + int(rescue_usage.get("output_tokens", 0))
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


def suggest_feed_seed_sites(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
    model: str,
    api_key: str,
) -> dict:
    existing_sources = existing_sources[:40]
    preferred_topics = [str(t).strip() for t in preferred_topics if str(t).strip()][:20]
    positive_examples = (positive_examples or [])[:8]
    negative_examples = (negative_examples or [])[:5]
    prompt = f"""あなたはRSSフィード探索アシスタントです。
既存の購読ソースと興味トピックを元に、「まだ登録していない可能性が高い」ニュース/技術メディアのサイトURL（ホームページURL）候補を提案してください。

要件:
- URLは実在しそうなサイトのトップURLを優先（https://example.com/ 形式）
- RSS URLを直接知らない場合はサイトトップURLでよい（後段でRSS探索する）
- 既存ソースと同じURLは除外
- 日本語で短い理由を付ける
- 最大30件
- JSONのみで返す

返却形式（必須）:
{{
  "items": [
    {{"url":"https://...", "reason":"..."}}
  ]
}}

Few-shot（好みの既存Feed例）:
{json.dumps(positive_examples, ensure_ascii=False)}

Few-shot（避けたい傾向の既存Feed例）:
{json.dumps(negative_examples, ensure_ascii=False)}

既存ソース:
{json.dumps(existing_sources, ensure_ascii=False)}

興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}
"""
    seed_schema = {
        "type": "OBJECT",
        "properties": {
            "items": {
                "type": "ARRAY",
                "items": {
                    "type": "OBJECT",
                    "properties": {
                        "url": {"type": "STRING"},
                        "reason": {"type": "STRING"},
                    },
                    "required": ["url", "reason"],
                },
            }
        },
        "required": ["items"],
    }
    text, usage = _generate_content(
        prompt,
        model=model,
        api_key=api_key,
        max_output_tokens=2200,
        response_schema=seed_schema,
    )
    data = _extract_first_json_object(text) or {}
    rows = data.get("items", [])
    if not isinstance(rows, list):
        rows = []
    existing_set = {_normalize_url_for_match(str(s.get("url") or "").strip()) for s in existing_sources}
    out: list[dict] = []
    for row in rows[:30]:
        if not isinstance(row, dict):
            continue
        url = str(row.get("url") or "").strip()
        reason = str(row.get("reason") or "").strip()[:180]
        if not url or _normalize_url_for_match(url) in existing_set:
            continue
        out.append({"url": url, "reason": reason})
    if len(out) == 0:
        rescue_prompt = f"""既存ソースと重複しないサイトURL候補を必ず10件以上返してください。JSONのみ。
{{
  "items": [
    {{"url":"https://...", "reason":"..."}}
  ]
}}
既存ソース:
{json.dumps(existing_sources, ensure_ascii=False)}
興味トピック:
{json.dumps(preferred_topics, ensure_ascii=False)}
"""
        rescue_text, rescue_usage = _generate_content(
            rescue_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=1800,
            response_schema=seed_schema,
        )
        rescue_data = _extract_first_json_object(rescue_text) or {}
        rescue_rows = rescue_data.get("items", [])
        if isinstance(rescue_rows, list):
            for row in rescue_rows[:30]:
                if not isinstance(row, dict):
                    continue
                url = str(row.get("url") or "").strip()
                reason = str(row.get("reason") or "").strip()[:180]
                if not url or _normalize_url_for_match(url) in existing_set:
                    continue
                out.append({"url": url, "reason": reason})
        usage["input_tokens"] = int(usage.get("input_tokens", 0)) + int(rescue_usage.get("input_tokens", 0))
        usage["output_tokens"] = int(usage.get("output_tokens", 0)) + int(rescue_usage.get("output_tokens", 0))
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


def extract_facts(title: str | None, content: str, model: str, api_key: str) -> dict:
    system_instruction = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
提供される記事から重要な事実を8〜18個の箇条書きで抽出してください。

# Rules
- 出力は必ず ["事実1", "事実2", ...] のJSON形式の配列のみとしてください。
- 余計な挨拶や解説は一切不要です。
- 事実は客観的かつ具体的に記述してください。
- 記事が英語の場合も、出力は自然な日本語にしてください。
- 固有名詞は原文を尊重し、適宜英字を維持してください。
"""

    prompt = f"""# Input
タイトル: {title or "（不明）"}

本文:
{content}
"""
    text, usage = _generate_content(prompt, model=model, api_key=api_key, max_output_tokens=1500, system_instruction=system_instruction)
    facts = _parse_json_string_array(text)
    return {"facts": facts, "llm": _llm_meta(model, "facts", usage)}


def summarize(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    model: str = "gemini-2.5-flash",
    api_key: str = "",
) -> dict:
    target_chars = _target_summary_chars(source_text_chars, facts)
    min_chars = _clamp_int(round(target_chars * 0.8), 160, 1000)
    max_chars = _clamp_int(round(target_chars * 1.2), 260, 1400)
    max_tokens = _summary_max_tokens(target_chars)
    facts_text = "\n".join(f"- {f}" for f in facts)
    system_instruction = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
与えられた事実リストから記事要約を作成してください。

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみにしてください。
- 前置き・後置き・コードフェンス・注釈は不要です。
- 要約は客観的・中立的な自然な日本語で書いてください。
- 記事の主題、何が起きたか、重要なポイントを過不足なく含めてください。
- 箇条書きではなく2〜4段落の文章でまとめてください。
- タイトルが主に英語の場合のみ translated_title に自然な日本語訳を入れてください。
- タイトルが日本語の場合は translated_title を空文字にしてください。
- 事実リストにない推測の断定、誇張表現、主観的評価は禁止です。
- topics は重複を避け、粒度を揃えてください。
- score_reason は採点の根拠を1〜2文で簡潔に述べてください。

# Output
{
  "summary": "要約",
  "topics": ["トピック1", "トピック2"],
  "translated_title": "英語タイトルの場合のみ日本語訳（日本語記事は空文字）",
  "score_breakdown": {
    "importance": 0.0〜1.0,
    "novelty": 0.0〜1.0,
    "actionability": 0.0〜1.0,
    "reliability": 0.0〜1.0,
    "relevance": 0.0〜1.0
  },
  "score_reason": "採点理由（1〜2文）"
}"""
    prompt = f"""# Input
summary は {min_chars}〜{max_chars}字程度で作成し、目標は約{target_chars}字にしてください。

タイトル: {title or "（不明）"}
事実:
{facts_text}
"""
    api_key_hash = hashlib.sha256((api_key or "").encode("utf-8")).hexdigest()[:16]
    cache_key = _cache_key_hash([_normalize_model_name(model), "summary-v2", api_key_hash, system_instruction])
    text, usage = _generate_content(
        prompt,
        model=model,
        api_key=api_key,
        max_output_tokens=max_tokens,
        system_instruction=system_instruction,
        context_cache_key=cache_key,
    )
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    topics = data.get("topics", [])
    if not isinstance(topics, list):
        topics = []
    score_breakdown = data.get("score_breakdown", {})
    if not isinstance(score_breakdown, dict):
        score_breakdown = {}
    score_breakdown = {
        "importance": _clamp01(score_breakdown.get("importance", 0.5)),
        "novelty": _clamp01(score_breakdown.get("novelty", 0.5)),
        "actionability": _clamp01(score_breakdown.get("actionability", 0.5)),
        "reliability": _clamp01(score_breakdown.get("reliability", 0.5)),
        "relevance": _clamp01(score_breakdown.get("relevance", 0.5)),
    }
    score_reason = str(data.get("score_reason") or "").strip()
    if not score_reason:
        score_reason = "総合的な重要度・新規性・実用性を基に採点。"
    translated_title = str(data.get("translated_title") or "").strip()
    if _needs_title_translation(title, translated_title):
        translated_title = _translate_title_to_ja(title or "", model=model, api_key=api_key)
    return {
        "summary": str(data.get("summary", "")).strip(),
        "topics": [str(t) for t in topics],
        "translated_title": translated_title[:300],
        "score": _summary_composite_score(score_breakdown),
        "score_breakdown": score_breakdown,
        "score_reason": score_reason[:400],
        "score_policy_version": "v2",
        "llm": _llm_meta(model, "summary", usage),
    }


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
    text, usage = _generate_content(
        summary_faithfulness_prompt(title, facts, summary),
        model=model,
        api_key=api_key,
        max_output_tokens=320,
        system_instruction=summary_faithfulness_system_instruction(),
        response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
    )
    result = normalize_summary_faithfulness_result(_extract_first_json_object(text))
    result["llm"] = _llm_meta(model, "faithfulness_check", usage)
    return result


def translate_title(title: str, model: str = "gemini-2.5-flash", api_key: str = "") -> dict:
    src = (title or "").strip()
    if not src:
        return {"translated_title": "", "llm": None}
    try:
        translated = _translate_title_to_ja(src, model=model, api_key=api_key)
    except Exception as e:
        fallback_model = "gemini-2.5-flash-lite"
        if _normalize_model_family(model) == fallback_model:
            raise
        _log.warning("gemini translate_title failed with model=%s, fallback=%s, err=%s", model, fallback_model, e)
        translated = _translate_title_to_ja(src, model=fallback_model, api_key=api_key)
    return {
        "translated_title": translated[:300],
        "llm": None,
    }


def compose_digest(digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
        }
    input_mode, digest_input = _build_digest_input_sections(items)
    system_instruction = """# Role
あなたはニュースダイジェスト編集者です。

# Task
与えられた記事一覧をもとに、メール用のダイジェスト本文を日本語で作成してください。

# Rules
- 当日分の全記事要約を踏まえて全体像をまとめてください。記事を取りこぼさないでください。
- 本文は900〜2200字程度を目安とし、必要なら超えて構いません。
- 本文は必ず次の順序で構成してください:
  1) 全体サマリ（1〜3段落）
  2) 注目ポイント（5〜10個。各ポイントは1〜2段落）
  3) その他のポイント（個数指定なし。箇条書き）
  4) 明日以降のフォローポイント（1段落）
  5) 締めの1文
- body は可読性を最優先し、各セクションの間に必ず空行1行（\\n\\n）を入れてください。
- 段落同士も必要に応じて空行（\\n\\n）で分けてください。
- 誇張せず、与えられた情報だけで書いてください。
- 出力はJSONオブジェクトのみとしてください。
"""

    prompt = f"""# Output
{{
  "subject": "件名（40字程度）",
  "body": "メール本文（プレーンテキスト。改行を含めてよい）",
  "sections": {{
    "overall_summary": "1〜3段落",
    "highlights": ["注目ポイント1（1〜2段落）", "注目ポイント2（1〜2段落）"],
    "other_points": ["補足1", "補足2"],
    "follow_up": "明日以降のフォローポイント（1段落）",
    "closing": "締めの1文"
  }}
}}

# Input
digest_date: {digest_date}
items_count: {len(items)}
input_mode: {input_mode}
items:
{digest_input}
"""
    digest_schema = {
        "type": "OBJECT",
        "properties": {
            "subject": {"type": "STRING"},
            "body": {"type": "STRING"},
            "sections": {
                "type": "OBJECT",
                "properties": {
                    "overall_summary": {"type": "STRING"},
                    "highlights": {"type": "ARRAY", "items": {"type": "STRING"}},
                    "other_points": {"type": "ARRAY", "items": {"type": "STRING"}},
                    "follow_up": {"type": "STRING"},
                    "closing": {"type": "STRING"},
                },
            },
        },
        "required": ["subject", "body"],
    }

    compose_timeout = _env_timeout_seconds("GEMINI_COMPOSE_DIGEST_TIMEOUT_SEC", 240.0)
    last_text = ""
    last_error = "unknown"
    for max_tokens in (10000, 15000):
        text, usage = _generate_content(
            prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=max_tokens,
            response_schema=digest_schema,
            timeout_sec=compose_timeout,
            system_instruction=system_instruction,
        )
        last_text = text
        subject, body = _extract_compose_digest_fields(text)
        if not subject or not body:
            last_error = "missing subject/body"
            continue
        if len(body) < 80:
            last_error = f"body too short: len={len(body)}"
            continue
        return {
            "subject": subject,
            "body": body,
            "llm": _llm_meta(model, "digest", usage),
        }

    snippet = last_text[:500].replace("\n", "\\n")
    raise RuntimeError(f"gemini compose_digest parse failed: {last_error}; response_snippet={snippet}")


def ask_question(query: str, candidates: list[dict], model: str, api_key: str) -> dict:
    if not candidates:
        return {
            "answer": "該当する記事は見つかりませんでした。",
            "bullets": [],
            "citations": [],
            "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
        }

    lines: list[str] = []
    for idx, item in enumerate(candidates, start=1):
        title = item.get("translated_title") or item.get("title") or "（タイトルなし）"
        facts = [str(v).strip() for v in (item.get("facts") or []) if str(v).strip()]
        lines.append(
            f"- item_id={item.get('item_id')} | rank={idx} | title={title} | published_at={item.get('published_at') or ''} | "
            f"topics={', '.join(item.get('topics') or [])} | similarity={item.get('similarity')} | "
            f"summary={str(item.get('summary') or '')[:500]} | facts={' / '.join(facts[:4])[:400]}"
        )

    system_instruction = """# Role
あなたはRSSキュレーションアシスタントです。

# Task
与えられた候補記事だけを根拠に、日本語で質問へ回答してください。

# Rules
- 根拠は候補記事だけに限定してください。
- 候補記事から判断できないことは「候補記事からは判断できない」と明記してください。
- 出力はJSONオブジェクトのみとし、余計な説明文は書かないでください。
- answer は2〜3文にしてください。
- bullets は2〜3件にしてください。
- citations は2〜3件に絞ってください。
- citations は同じ話題に偏らせず、回答の主要な論点を支える記事を優先してください。
- answer の各文末には対応する item_id を [[item_id]] 形式で付けてください。
- bullets には citation マーカーを付けないでください。
- answer で使う [[item_id]] は citations に含まれる item_id だけを使ってください。
- [[item_id]] を付けられない文は書かないでください。
"""

    prompt = f"""# Output
{{
  "answer": "2〜3文の回答 [[item_id]]",
  "bullets": ["補足ポイント1", "補足ポイント2"],
  "citations": [
    {{"item_id": "uuid", "reason": "この観点の根拠"}}
  ]
}}

# Input
question: {query}
candidates:
{chr(10).join(lines)}
"""
    ask_schema = {
        "type": "OBJECT",
        "properties": {
            "answer": {"type": "STRING"},
            "bullets": {"type": "ARRAY", "items": {"type": "STRING"}},
            "citations": {
                "type": "ARRAY",
                "items": {
                    "type": "OBJECT",
                    "properties": {
                        "item_id": {"type": "STRING"},
                        "reason": {"type": "STRING"},
                    },
                    "required": ["item_id"],
                },
            },
        },
        "required": ["answer", "citations"],
    }
    text, usage = _generate_content(
        prompt,
        model=model,
        api_key=api_key,
        max_output_tokens=3200,
        response_schema=ask_schema,
        timeout_sec=_env_timeout_seconds("GEMINI_TIMEOUT_SEC", 90.0),
        system_instruction=system_instruction,
    )
    data = _extract_first_json_object(text) or {}
    answer = str(data.get("answer") or "").strip()
    if not answer:
        s = _strip_code_fence(text)
        m_answer = re.search(r'"answer"\s*:\s*"((?:\\.|[^"\\])*)"', s, re.S)
        if m_answer:
            answer = _decode_json_string_fragment(m_answer.group(1)).strip()
    if not answer:
        answer = _extract_json_string_value_loose(text, "answer")
    bullets = [str(v).strip() for v in (data.get("bullets") or []) if str(v).strip()]
    citations = []
    for raw in data.get("citations") or []:
        if not isinstance(raw, dict):
            continue
        item_id = str(raw.get("item_id") or "").strip()
        if not item_id:
            continue
        citations.append({
            "item_id": item_id,
            "reason": str(raw.get("reason") or "").strip(),
        })
    if not citations:
        s = _strip_code_fence(text)
        for match in re.finditer(r'"item_id"\s*:\s*"([^"]+)"(?:[^}]*"reason"\s*:\s*"((?:\\.|[^"\\])*)")?', s, re.S):
            citations.append({
                "item_id": match.group(1).strip(),
                "reason": _decode_json_string_fragment(match.group(2)).strip() if match.group(2) else "",
            })
    if not answer:
        raise RuntimeError(f"gemini ask missing answer; response_snippet={text[:500]}")
    if len(citations) < min(3, len(candidates)):
        seen = {str(c.get("item_id") or "").strip() for c in citations}
        for item in candidates:
            item_id = str(item.get("item_id") or "").strip()
            if not item_id or item_id in seen:
                continue
            citations.append({
                "item_id": item_id,
                "reason": "回答に関連する候補記事",
            })
            seen.add(item_id)
            if len(citations) >= min(5, len(candidates)):
                break
    return {
        "answer": answer,
        "bullets": bullets[:3],
        "citations": citations[:3],
        "llm": _llm_meta(model, "digest", usage),
    }


def compose_digest_cluster_draft(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    model: str,
    api_key: str,
) -> dict:
    prompt = f"""あなたはニュースダイジェストの下書き編集者です。
以下は同じ話題（クラスタ）に属する複数記事の要点メモです。重複をまとめ、事実ベースで読みやすいクラスタ下書きに整理してください。

要件:
- 与えられた内容のみ使う（推測しない）
- 重複をまとめる
- 重要な相違点があれば残す
- プレーンテキストで返す
- 箇条書き 3〜8 行程度
- JSONのみで返す

返却形式:
{{
  "draft_summary": "- ...\\n- ..."
}}

cluster_label: {cluster_label}
item_count: {item_count}
topics: {json.dumps(topics or [], ensure_ascii=False)}
source_lines:
{json.dumps(source_lines or [], ensure_ascii=False)}
"""
    text, usage = _generate_content(prompt, model=model, api_key=api_key, max_output_tokens=900)
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    summary = str(data.get("draft_summary") or "").strip()
    if not summary:
        summary = "\n".join(source_lines[:5])
    return {
        "draft_summary": summary,
        "llm": _llm_meta(model, "digest_cluster_draft", usage),
    }
