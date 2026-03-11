import json
import logging
import os
import re
import time
import anthropic
from app.services.llm_catalog import model_pricing
from app.services.llm_text_utils import (
    clamp01 as _clamp01,
    clamp_int as _clamp_int,
    decode_json_string_fragment as _decode_json_string_fragment,
    extract_compose_digest_fields as _extract_compose_digest_fields,
    extract_first_json_object as _extract_first_json_object,
    extract_json_string_value_loose as _extract_json_string_value_loose,
    parse_json_string_array as _parse_json_string_array,
    strip_code_fence as _strip_code_fence,
    summary_composite_score as _summary_composite_score,
    summary_max_tokens as _summary_max_tokens,
)
from app.services.summary_faithfulness_common import (
    SUMMARY_FAITHFULNESS_SCHEMA,
    summary_faithfulness_prompt,
    summary_faithfulness_retry_prompt,
    summary_faithfulness_system_instruction,
)
from app.services.summary_faithfulness_runner import run_summary_faithfulness_check
from app.services.facts_check_common import (
    FACTS_CHECK_SCHEMA,
    facts_check_prompt,
    facts_check_retry_prompt,
    facts_check_system_instruction,
)
from app.services.facts_check_runner import run_facts_check
from app.services.summary_task_common import build_summary_task
from app.services.summary_parse_common import finalize_summary_result
from app.services.title_translation_common import run_title_translation

_client = None
_facts_model = os.getenv("ANTHROPIC_FACTS_MODEL", "claude-haiku-4-5")
_summary_model = os.getenv("ANTHROPIC_SUMMARY_MODEL", "claude-sonnet-4-6")
_summary_model_fallback = os.getenv("ANTHROPIC_SUMMARY_MODEL_FALLBACK", "claude-sonnet-4-5-20250929")
_facts_model_fallback = os.getenv("ANTHROPIC_FACTS_MODEL_FALLBACK", "claude-3-5-haiku-20241022")
_digest_model = os.getenv("ANTHROPIC_DIGEST_MODEL", _summary_model)
_digest_model_fallback = os.getenv("ANTHROPIC_DIGEST_MODEL_FALLBACK", _summary_model_fallback)
_feed_suggest_model = os.getenv("ANTHROPIC_FEED_SUGGEST_MODEL", _summary_model)
_feed_suggest_model_fallback = os.getenv("ANTHROPIC_FEED_SUGGEST_MODEL_FALLBACK", _summary_model_fallback)
_log = logging.getLogger(__name__)
_ANTHROPIC_PRICING_SOURCE_VERSION = "anthropic_static_2026_02"

_LEGACY_MODEL_PRICING = {
    # USD per 1M tokens (Claude API pricing); cache write assumes 5m cache.
    "claude-haiku-4-5": {
        "input_per_mtok_usd": 1.0,
        "output_per_mtok_usd": 5.0,
        "cache_write_per_mtok_usd": 1.25,
        "cache_read_per_mtok_usd": 0.10,
    },
    "claude-sonnet-4-6": {
        "input_per_mtok_usd": 3.0,
        "output_per_mtok_usd": 15.0,
        "cache_write_per_mtok_usd": 3.75,
        "cache_read_per_mtok_usd": 0.30,
    },
    "claude-opus-4-6": {
        "input_per_mtok_usd": 5.0,
        "output_per_mtok_usd": 25.0,
        "cache_write_per_mtok_usd": 6.25,
        "cache_read_per_mtok_usd": 0.50,
    },
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


def _split_text_chunks(text: str, chunk_chars: int = 8000, overlap_chars: int = 400) -> list[str]:
    text = (text or "").strip()
    if not text:
        return []
    if len(text) <= chunk_chars:
        return [text]

    chunks: list[str] = []
    start = 0
    n = len(text)
    while start < n:
        end = min(n, start + chunk_chars)
        chunks.append(text[start:end])
        if end >= n:
            break
        start = max(end - overlap_chars, start + 1)
    return chunks


def _translate_title_to_ja(title: str, model: str, api_key: str | None = None) -> str:
    prompt = f"""次の英語タイトルを自然な日本語に翻訳してください。
JSONで返してください:
{{
  "translated_title": "日本語タイトル"
}}

タイトル: {title}
"""
    plain_prompt = f"""次のタイトルを日本語に翻訳してください。
説明は不要です。翻訳結果のみを1行で返してください。

タイトル: {title}
"""
    return run_title_translation(
        title,
        structured_call=lambda: (
            lambda message: (
                str(((_extract_first_json_object(message.content[0].text.strip()) or {}).get("translated_title") or ""))
                if message is not None
                else ""
            )
        )(
            _call_with_model_fallback(
                prompt,
                model,
                _summary_model_fallback,
                max_tokens=200,
                api_key=api_key,
            )[0]
        ),
        plain_retry_call=lambda: (
            lambda message: (message.content[0].text.strip() if message is not None else "")
        )(
            _call_with_model_fallback(
                plain_prompt,
                model,
                _summary_model_fallback,
                max_tokens=120,
                api_key=api_key,
            )[0]
        ),
    )


def _merge_fact_lists(fact_lists: list[list[str]], max_items: int = 24) -> list[str]:
    # De-dup while preserving coverage across chunks by interleaving.
    normalized_seen: set[str] = set()
    merged: list[str] = []
    max_len = max((len(xs) for xs in fact_lists), default=0)
    for i in range(max_len):
        for facts in fact_lists:
            if i >= len(facts):
                continue
            fact = facts[i].strip()
            if not fact:
                continue
            key = " ".join(fact.lower().split())
            if key in normalized_seen:
                continue
            normalized_seen.add(key)
            merged.append(fact)
            if len(merged) >= max_items:
                return merged
    return merged


def _merge_llm_metas(metas: list[dict], purpose: str) -> dict:
    valid = [m for m in metas if isinstance(m, dict)]
    if not valid:
        return {
            "provider": "none",
            "model": "none",
            "pricing_model_family": "",
            "pricing_source": _ANTHROPIC_PRICING_SOURCE_VERSION,
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_creation_input_tokens": 0,
            "cache_read_input_tokens": 0,
            "estimated_cost_usd": 0.0,
        }

    providers = {str(m.get("provider", "")) for m in valid}
    models = [str(m.get("model", "")) for m in valid if m.get("model")]
    families = {str(m.get("pricing_model_family", "")) for m in valid if m.get("pricing_model_family") is not None}
    sources = {str(m.get("pricing_source", "default")) for m in valid}

    return {
        "provider": next(iter(providers)) if len(providers) == 1 else "mixed",
        "model": models[0] if len(set(models)) == 1 and models else "multiple",
        "pricing_model_family": next(iter(families)) if len(families) == 1 else "mixed",
        "pricing_source": next(iter(sources)) if len(sources) == 1 else "mixed",
        "input_tokens": sum(int(m.get("input_tokens", 0) or 0) for m in valid),
        "output_tokens": sum(int(m.get("output_tokens", 0) or 0) for m in valid),
        "cache_creation_input_tokens": sum(int(m.get("cache_creation_input_tokens", 0) or 0) for m in valid),
        "cache_read_input_tokens": sum(int(m.get("cache_read_input_tokens", 0) or 0) for m in valid),
        "estimated_cost_usd": round(sum(float(m.get("estimated_cost_usd", 0.0) or 0.0) for m in valid), 8),
        "calls": len(valid),
        "purpose": purpose,
    }


def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _normalize_model_family(model: str) -> str:
    if not model:
        return ""
    if model_pricing(model) is not None:
        return model
    for family in sorted(_LEGACY_MODEL_PRICING.keys(), key=len, reverse=True):
        if model == family or model.startswith(family + "-"):
            return family
    return model


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
                "cache_write_per_mtok_usd": 0.0,
                "cache_read_per_mtok_usd": 0.0,
            },
        )
    )
    source = str(base.get("pricing_source") or _ANTHROPIC_PRICING_SOURCE_VERSION)
    # Optional per-purpose overrides for temporary pricing changes without deploy.
    prefix = f"ANTHROPIC_{purpose.upper()}_"
    override_map = {
        "input_per_mtok_usd": _env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
        "output_per_mtok_usd": _env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
        "cache_write_per_mtok_usd": _env_optional_float(prefix + "CACHE_WRITE_PER_MTOK_USD"),
        "cache_read_per_mtok_usd": _env_optional_float(prefix + "CACHE_READ_PER_MTOK_USD"),
    }
    for k, v in override_map.items():
        if v is not None:
            base[k] = v
            source = "env_override"
    base["pricing_source"] = source
    base["pricing_model_family"] = family
    return base


def _message_usage(message) -> dict:
    usage = getattr(message, "usage", None)
    if usage is None:
        return {
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_creation_input_tokens": 0,
            "cache_read_input_tokens": 0,
        }
    return {
        "input_tokens": int(getattr(usage, "input_tokens", 0) or 0),
        "output_tokens": int(getattr(usage, "output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(getattr(usage, "cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(getattr(usage, "cache_read_input_tokens", 0) or 0),
    }


def _estimate_cost_usd(model: str, purpose: str, usage: dict) -> float:
    p = _pricing_for_model(model, purpose)
    total = 0.0
    total += usage["input_tokens"] / 1_000_000 * p["input_per_mtok_usd"]
    total += usage["output_tokens"] / 1_000_000 * p["output_per_mtok_usd"]
    total += usage["cache_creation_input_tokens"] / 1_000_000 * p["cache_write_per_mtok_usd"]
    total += usage["cache_read_input_tokens"] / 1_000_000 * p["cache_read_per_mtok_usd"]
    return round(total, 8)


def _llm_meta(message, purpose: str, model: str, provider: str = "anthropic") -> dict:
    usage = _message_usage(message) if message is not None else _message_usage(None)
    actual_model = str(getattr(message, "model", None) or model)
    pricing = _pricing_for_model(actual_model, purpose)
    return {
        "provider": provider,
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", "default"),
        **usage,
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }


def _client_for_api_key(api_key: str | None):
    if api_key:
        return anthropic.Anthropic(api_key=api_key)
    return None


def _messages_create(
    prompt: str,
    model: str,
    max_tokens: int = 1024,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
):
    client = _client_for_api_key(api_key)
    if client is None:
        return None
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("ANTHROPIC_TIMEOUT_SEC", 90.0)
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "timeout": req_timeout,
    }
    if system_prompt is not None:
        system_block: dict = {"type": "text", "text": system_prompt}
        if enable_prompt_cache:
            system_block["cache_control"] = {"type": "ephemeral"}
            kwargs["extra_headers"] = {"anthropic-beta": "prompt-caching-2024-07-31"}
        kwargs["system"] = [system_block]
        kwargs["messages"] = [{"role": "user", "content": user_prompt or prompt}]
    else:
        kwargs["messages"] = [{"role": "user", "content": prompt}]
    return client.messages.create(**kwargs)


def _is_rate_limit_error(err: Exception) -> bool:
    s = str(err).lower()
    return "429" in s or "rate_limit" in s


def _call_with_retries(
    prompt: str,
    model: str,
    max_tokens: int,
    retries: int = 2,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
):
    last_err = None
    for attempt in range(retries + 1):
        try:
            return _messages_create(
                prompt,
                model,
                max_tokens=max_tokens,
                api_key=api_key,
                timeout_sec=timeout_sec,
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                enable_prompt_cache=enable_prompt_cache,
            )
        except Exception as e:
            last_err = e
            if attempt >= retries or not _is_rate_limit_error(e):
                raise
            # Small exponential backoff for rate limits.
            sleep_sec = 1.0 * (2 ** attempt)
            _log.warning(
                "anthropic rate-limited model=%s retry_in=%.1fs attempt=%d/%d",
                model,
                sleep_sec,
                attempt + 1,
                retries + 1,
            )
            time.sleep(sleep_sec)
    if last_err is not None:
        raise last_err
    return None


def _call_with_model_fallback(
    prompt: str,
    primary_model: str,
    fallback_model: str | None,
    max_tokens: int = 1024,
    api_key: str | None = None,
    timeout_sec: float | None = None,
    system_prompt: str | None = None,
    user_prompt: str | None = None,
    enable_prompt_cache: bool = False,
):
    if _client_for_api_key(api_key) is None:
        return None, None
    try:
        return (
            _call_with_retries(
                prompt,
                primary_model,
                max_tokens=max_tokens,
                api_key=api_key,
                timeout_sec=timeout_sec,
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                enable_prompt_cache=enable_prompt_cache,
            ),
            primary_model,
        )
    except Exception as e:
        _log.warning("anthropic call failed model=%s err=%s", primary_model, e)
        if fallback_model and fallback_model != primary_model:
            try:
                return (
                    _call_with_retries(
                        prompt,
                        fallback_model,
                        max_tokens=max_tokens,
                        api_key=api_key,
                        timeout_sec=timeout_sec,
                        system_prompt=system_prompt,
                        user_prompt=user_prompt,
                        enable_prompt_cache=enable_prompt_cache,
                    ),
                    fallback_model,
                )
            except Exception as e2:
                _log.warning("anthropic fallback failed model=%s err=%s", fallback_model, e2)
        return None, None


def extract_facts(title: str | None, content: str, api_key: str | None = None, model: str | None = None) -> dict:
    if _client_for_api_key(api_key) is None:
        lines = [line.strip() for line in content.splitlines() if line.strip()]
        facts = lines[:5]
        if not facts and title:
            facts = [f"タイトル: {title}"]
        return {
            "facts": facts,
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    chunks = _split_text_chunks(content, chunk_chars=8000, overlap_chars=400)
    if not chunks:
        return {"facts": [], "llm": _merge_llm_metas([], "facts")}

    all_fact_lists: list[list[str]] = []
    llm_metas: list[dict] = []
    any_llm_success = False
    per_chunk_fact_target = 4 if len(chunks) <= 3 else 3 if len(chunks) <= 8 else 2

    for idx, chunk in enumerate(chunks, start=1):
        system_prompt = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
提供される記事チャンクから重要な事実を{per_chunk_fact_target}〜{per_chunk_fact_target + 2}個の箇条書きで抽出してください。

# Rules
- 出力は必ず ["事実1", "事実2", ...] のJSON形式の配列のみとしてください。
- 余計な挨拶や解説は一切不要です。
- このチャンク内に明示されている内容だけを対象にしてください。
- 事実は客観的かつ具体的に記述してください。
- 記事が英語の場合も、出力は自然な日本語にしてください。
- 固有名詞は原文を尊重し、適宜英字を維持してください。
"""

        user_prompt = f"""# Input
タイトル: {title or "（不明）"}
チャンク: {idx}/{len(chunks)}

本文:
{chunk}
"""
        message, used_model = _call_with_model_fallback(
            f"{system_prompt}\n\n{user_prompt}",
            str(model or _facts_model),
            _facts_model_fallback,
            max_tokens=1024,
            api_key=api_key,
            system_prompt=system_prompt,
            user_prompt=user_prompt,
        )
        if message is None:
            continue
        any_llm_success = True
        text = message.content[0].text.strip()
        all_fact_lists.append(_parse_json_string_array(text))
        llm_metas.append(_llm_meta(message, "facts", used_model or _facts_model))

    if not any_llm_success:
        lines = [line.strip() for line in content.splitlines() if line.strip()]
        return {
            "facts": lines[:5],
            "llm": {
                "provider": "local-fallback",
                "model": _facts_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    merged_facts = _merge_fact_lists(all_fact_lists, max_items=24)
    llm = _merge_llm_metas(llm_metas, "facts")
    llm["chunk_count"] = len(chunks)
    llm["chunk_success_count"] = len(llm_metas)
    return {
        "facts": merged_facts,
        "llm": llm,
    }


def summarize(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    task = build_summary_task(title, facts, source_text_chars)
    max_tokens = _summary_max_tokens(task["target_chars"])

    if _client_for_api_key(api_key) is None:
        summary = " / ".join(facts[:8])[:task["max_chars"]] if facts else (title or "")
        score_breakdown = {
            "importance": 0.4,
            "novelty": 0.4,
            "actionability": 0.4,
            "reliability": 0.5,
            "relevance": 0.5,
        }
        return {
            "summary": summary or "要約を生成できませんでした",
            "topics": ["local-dev"],
            "translated_title": "",
            "score": _summary_composite_score(score_breakdown),
            "score_breakdown": score_breakdown,
            "score_reason": "ローカルフォールバックのため簡易スコアです。",
            "score_policy_version": "v2",
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    prompt = f"{task['system_instruction']}\n\n{task['prompt']}"
    enable_summary_prompt_cache = os.getenv("ANTHROPIC_SUMMARY_PROMPT_CACHE", "1").strip() not in ("0", "false", "False")
    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _summary_model),
        _summary_model_fallback,
        max_tokens=max_tokens,
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
        enable_prompt_cache=enable_summary_prompt_cache,
    )
    if message is None:
        summary = " / ".join(facts[:8])[:task["max_chars"]] if facts else (title or "")
        score_breakdown = {
            "importance": 0.4,
            "novelty": 0.4,
            "actionability": 0.4,
            "reliability": 0.5,
            "relevance": 0.5,
        }
        return {
            "summary": summary or "要約を生成できませんでした",
            "topics": ["local-dev"],
            "translated_title": "",
            "score": _summary_composite_score(score_breakdown),
            "score_breakdown": score_breakdown,
            "score_reason": "Anthropic応答を取得できなかったため簡易スコアです。",
            "score_policy_version": "v2",
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _summary_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    text = message.content[0].text.strip()
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    topics = data.get("topics", [])
    if not isinstance(topics, list):
        topics = []
    return finalize_summary_result(
        title=title,
        summary_text=str(data.get("summary", "")),
        topics=topics,
        raw_score_breakdown=data.get("score_breakdown") if isinstance(data.get("score_breakdown"), dict) else {},
        score_reason=str(data.get("score_reason") or "").strip(),
        translated_title=str(data.get("translated_title") or "").strip(),
        translate_func=lambda raw_title: _translate_title_to_ja(raw_title, used_model or _summary_model, api_key=api_key),
        llm=_llm_meta(message, "summary", used_model or _summary_model),
        error_prefix="anthropic summarize parse failed",
        response_text=text,
    )


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, api_key: str | None = None, model: str | None = None) -> dict:
    prompt = summary_faithfulness_prompt(title, facts, summary)
    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _summary_model),
        _summary_model_fallback,
        max_tokens=320,
        api_key=api_key,
        system_prompt=summary_faithfulness_system_instruction(),
        user_prompt=prompt,
    )
    if message is None:
        result = {"verdict": "warn", "short_comment": "判定モデル応答を取得できなかったため簡易扱いです。"}
        result["llm"] = {
            "provider": "local-fallback",
            "model": used_model or _summary_model,
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_creation_input_tokens": 0,
            "cache_read_input_tokens": 0,
            "estimated_cost_usd": 0.0,
        }
        return result
    return run_summary_faithfulness_check(
        lambda: (
            message.content[0].text.strip(),
            _llm_meta(message, "faithfulness_check", used_model or _summary_model),
        ),
        retry_call=lambda: (
            lambda retry_message, retry_model: (
                (retry_message.content[0].text.strip() if retry_message is not None else ""),
                _llm_meta(retry_message, "faithfulness_check", retry_model or used_model or _summary_model)
                if retry_message is not None
                else {
                    "provider": "anthropic",
                    "model": retry_model or used_model or _summary_model,
                    "pricing_model_family": retry_model or used_model or _summary_model,
                    "pricing_source": _ANTHROPIC_PRICING_SOURCE_VERSION,
                    "input_tokens": 0,
                    "output_tokens": 0,
                    "cache_creation_input_tokens": 0,
                    "cache_read_input_tokens": 0,
                    "estimated_cost_usd": 0.0,
                },
            )
        )(
            *_call_with_model_fallback(
                summary_faithfulness_retry_prompt(title, facts, summary),
                str(model or _summary_model),
                _summary_model_fallback,
                max_tokens=120,
                api_key=api_key,
                system_prompt="pass / warn / fail のいずれか1語のみを返す。",
                user_prompt=summary_faithfulness_retry_prompt(title, facts, summary),
                enable_prompt_cache=False,
            )
        ),
    )


def check_facts(title: str | None, content: str, facts: list[str], api_key: str | None = None, model: str | None = None) -> dict:
    prompt = facts_check_prompt(title, content, facts)
    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _summary_model),
        _summary_model_fallback,
        max_tokens=320,
        api_key=api_key,
        system_prompt=facts_check_system_instruction(),
        user_prompt=prompt,
    )
    if message is None:
        return {
            "verdict": "warn",
            "short_comment": "判定モデル応答を取得できなかったため簡易扱いです。",
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _summary_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    retry_prompt = facts_check_retry_prompt(title, content, facts)
    return run_facts_check(
        lambda: (
            message.content[0].text.strip(),
            _llm_meta(message, "facts_check", used_model or _summary_model),
        ),
        retry_call=lambda: (
            lambda retry_message, retry_model: (
                (retry_message.content[0].text.strip() if retry_message is not None else ""),
                _llm_meta(retry_message, "facts_check", retry_model or used_model or _summary_model)
                if retry_message is not None
                else {
                    "provider": "anthropic",
                    "model": retry_model or used_model or _summary_model,
                    "pricing_model_family": retry_model or used_model or _summary_model,
                    "pricing_source": _ANTHROPIC_PRICING_SOURCE_VERSION,
                    "input_tokens": 0,
                    "output_tokens": 0,
                    "cache_creation_input_tokens": 0,
                    "cache_read_input_tokens": 0,
                    "estimated_cost_usd": 0.0,
                },
            )
        )(
            *_call_with_model_fallback(
                retry_prompt,
                str(model or _summary_model),
                _summary_model_fallback,
                max_tokens=120,
                api_key=api_key,
                system_prompt="pass / warn / fail のいずれか1語のみを返す。",
                user_prompt=retry_prompt,
                enable_prompt_cache=False,
            )
        ),
    )


def translate_title(title: str, api_key: str | None = None, model: str | None = None) -> dict:
    src = (title or "").strip()
    if not src:
        return {"translated_title": "", "llm": None}
    prompt = f"""次の英語タイトルを自然な日本語に翻訳してください。
JSONで返してください:
{{
  "translated_title": "日本語タイトル"
}}

タイトル: {src}
"""
    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _summary_model),
        _summary_model_fallback,
        max_tokens=200,
        api_key=api_key,
    )
    if message is None:
        return {"translated_title": _translate_title_to_ja(src, str(model or _summary_model), api_key=api_key), "llm": None}

    text = message.content[0].text.strip()
    data = _extract_first_json_object(text) or {}
    translated = str(data.get("translated_title") or "").strip()
    if not translated:
        translated = _translate_title_to_ja(src, used_model or _summary_model, api_key=api_key)
    return {
        "translated_title": translated[:300],
        "llm": _llm_meta(message, "summary", used_model or _summary_model),
    }


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
    # Small/medium days: preserve per-item details.
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

    # Large days: topic-based compression + top item highlights.
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


def compose_digest(digest_date: str, items: list[dict], api_key: str | None = None, model: str | None = None) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": {
                "provider": "none",
                "model": "none",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    input_mode, digest_input = _build_digest_input_sections(items)
    compose_timeout = _env_timeout_seconds("ANTHROPIC_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0)

    system_prompt = """# Role
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

    user_prompt = f"""# Output
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

    message, used_model = _call_with_model_fallback(
        f"{system_prompt}\n\n{user_prompt}",
        str(model or _digest_model),
        _digest_model_fallback,
        max_tokens=10000,
        api_key=api_key,
        timeout_sec=compose_timeout,
        system_prompt=system_prompt,
        user_prompt=user_prompt,
    )
    if message is None:
        top_topics = []
        for item in items:
            top_topics.extend(item.get("topics") or [])
        lines = [
            "【全体サマリ】",
            "本日のダイジェスト（当日分の全記事要約ベース）をお届けします。",
            "",
            "【注目ポイント】",
        ]
        lines += [
            f"- #{item.get('rank')} {item.get('title') or '（タイトルなし）'}"
            for item in items
        ]
        if top_topics:
            lines += ["", "【その他のポイント】", "主なトピック: " + ", ".join(dict.fromkeys(top_topics))]
        lines += ["", "【明日以降のフォローポイント】", "新規更新の続報と追加情報の有無を継続確認してください。", "", "以上です。"]
        body = "\n".join(lines)
        return {
            "subject": f"Sifto Digest {digest_date}",
            "body": body,
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _digest_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    text = message.content[0].text.strip()
    subject, body = _extract_compose_digest_fields(text)
    if not subject or not body:
        snippet = text[:500].replace("\n", "\\n")
        raise RuntimeError(f"claude compose_digest missing subject/body; response_snippet={snippet}")
    if len(body) < 80:
        raise RuntimeError(f"claude compose_digest body too short: len={len(body)}")
    llm = _llm_meta(message, "digest", used_model or _digest_model)
    llm["input_mode"] = input_mode
    llm["items_count"] = len(items)
    return {
        "subject": subject,
        "body": body,
        "llm": llm,
    }


def ask_question(query: str, candidates: list[dict], api_key: str | None = None, model: str | None = None) -> dict:
    if not candidates:
        return {
            "answer": "該当する記事は見つかりませんでした。",
            "bullets": [],
            "citations": [],
            "llm": {
                "provider": "none",
                "model": "none",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
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

    system_prompt = """# Role
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

    user_prompt = f"""# Output
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
    message, used_model = _call_with_model_fallback(
        f"{system_prompt}\n\n{user_prompt}",
        str(model or _digest_model),
        _digest_model_fallback,
        max_tokens=3200,
        api_key=api_key,
        timeout_sec=_env_timeout_seconds("ANTHROPIC_TIMEOUT_SEC", 90.0),
        system_prompt=system_prompt,
        user_prompt=user_prompt,
    )
    if message is None:
        return {
            "answer": "候補記事から回答を生成できませんでした。",
            "bullets": [],
            "citations": [],
            "llm": {
                "provider": "none",
                "model": used_model or _digest_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    text = message.content[0].text.strip()
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
        snippet = text[:500].replace("\n", "\\n")
        raise RuntimeError(f"claude ask missing answer; response_snippet={snippet}")
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
        "llm": _llm_meta(message, "digest", used_model or _digest_model),
    }


def compose_digest_cluster_draft(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    cluster_label = str(cluster_label or "話題").strip() or "話題"
    topics = [str(t).strip() for t in topics if str(t).strip()][:8]
    source_lines = [str(x).strip() for x in source_lines if str(x).strip()][:16]
    if not source_lines:
        return {
            "draft_summary": "",
            "llm": {
                "provider": "none",
                "model": "none",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    if _client_for_api_key(api_key) is None:
        return {
            "draft_summary": "\n".join(source_lines),
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

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
topics: {json.dumps(topics, ensure_ascii=False)}
source_lines:
{json.dumps(source_lines, ensure_ascii=False)}
"""

    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _digest_model),
        _digest_model_fallback,
        max_tokens=1200,
        api_key=api_key,
    )
    if message is None:
        return {
            "draft_summary": "\n".join(source_lines),
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _digest_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    text = message.content[0].text.strip()
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    draft_summary = str(data.get("draft_summary") or "").strip()
    if not draft_summary:
        draft_summary = "\n".join(source_lines)
    return {
        "draft_summary": draft_summary,
        "llm": _llm_meta(message, "digest_cluster_draft", used_model or _digest_model),
    }


def rank_feed_suggestions(
    existing_sources: list[dict],
    preferred_topics: list[str],
    candidates: list[dict],
    positive_examples: list[dict] | None = None,
    negative_examples: list[dict] | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    if not candidates:
        return {
            "items": [],
            "llm": {
                "provider": "none",
                "model": "none",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    # Keep prompt bounded.
    existing_sources = existing_sources[:40]
    preferred_topics = [str(t).strip() for t in preferred_topics if str(t).strip()][:20]
    candidates = candidates[:80]
    positive_examples = (positive_examples or [])[:8]
    negative_examples = (negative_examples or [])[:5]

    if _client_for_api_key(api_key) is None:
        # Local/dev fallback: keep order and synthesize simple reasons.
        out = []
        for c in candidates:
            reasons = c.get("reasons") or []
            matched_topics = c.get("matched_topics") or []
            reason = " / ".join([*(["高評価トピックに近い"] if matched_topics else []), *[str(r) for r in reasons[:1]]]) or "関連候補"
            out.append(
                {
                    "id": c.get("id"),
                    "url": c.get("url"),
                    "reason": reason[:120],
                    "confidence": 0.4 if matched_topics else 0.25,
                }
            )
        return {
            "items": out,
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

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

    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _feed_suggest_model),
        _feed_suggest_model_fallback,
        max_tokens=2800,
        api_key=api_key,
    )
    if message is None:
        return {
            "items": [],
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _feed_suggest_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

    text = message.content[0].text.strip()
    start = text.find("{")
    end = text.rfind("}") + 1
    try:
        data = json.loads(text[start:end])
    except Exception:
        data = {}
    rows = data.get("items", [])
    if not isinstance(rows, list):
        rows = []
    out: list[dict] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        cid = str(row.get("id") or "").strip()
        url = str(row.get("url") or "").strip()
        if not cid and not url:
            continue
        reason = str(row.get("reason") or "").strip()[:180]
        try:
            confidence = _clamp01(float(row.get("confidence", 0.5)), 0.5)
        except Exception:
            confidence = 0.5
        out.append({"id": cid or None, "url": url, "reason": reason, "confidence": confidence})
    return {
        "items": out,
        "llm": _llm_meta(message, "source_suggestion", used_model or _feed_suggest_model),
    }


def suggest_feed_seed_sites(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None = None,
    negative_examples: list[dict] | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    existing_sources = existing_sources[:40]
    preferred_topics = [str(t).strip() for t in preferred_topics if str(t).strip()][:20]
    positive_examples = (positive_examples or [])[:8]
    negative_examples = (negative_examples or [])[:5]

    if _client_for_api_key(api_key) is None:
        return {
            "items": [],
            "llm": {
                "provider": "local-dev",
                "model": "local-fallback",
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }

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
    message, used_model = _call_with_model_fallback(
        prompt,
        str(model or _feed_suggest_model),
        _feed_suggest_model_fallback,
        max_tokens=2200,
        api_key=api_key,
    )
    if message is None:
        return {
            "items": [],
            "llm": {
                "provider": "local-fallback",
                "model": used_model or _feed_suggest_model,
                "input_tokens": 0,
                "output_tokens": 0,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "estimated_cost_usd": 0.0,
            },
        }
    text = message.content[0].text.strip()
    data = _extract_first_json_object(text) or {}
    rows = data.get("items", [])
    if not isinstance(rows, list):
        rows = []
    existing_set = {str(s.get("url") or "").strip().lower().rstrip("/") for s in existing_sources}
    out: list[dict] = []
    for row in rows[:30]:
        if not isinstance(row, dict):
            continue
        url = str(row.get("url") or "").strip()
        reason = str(row.get("reason") or "").strip()[:180]
        if not url or url.lower().rstrip("/") in existing_set:
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
        rescue_message, _ = _call_with_model_fallback(
            rescue_prompt,
            str(model or _feed_suggest_model),
            _feed_suggest_model_fallback,
            max_tokens=1500,
            api_key=api_key,
        )
        if rescue_message is not None:
            rescue_data = _extract_first_json_object(rescue_message.content[0].text.strip()) or {}
            rescue_rows = rescue_data.get("items", [])
            if isinstance(rescue_rows, list):
                for row in rescue_rows[:30]:
                    if not isinstance(row, dict):
                        continue
                    url = str(row.get("url") or "").strip()
                    reason = str(row.get("reason") or "").strip()[:180]
                    if not url or url.lower().rstrip("/") in existing_set:
                        continue
                    out.append({"url": url, "reason": reason})
    return {
        "items": out,
        "llm": _llm_meta(message, "source_suggestion", used_model or _feed_suggest_model),
    }
