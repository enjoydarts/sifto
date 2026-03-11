import json
import logging
import os
import re
import time

import httpx

from app.services.llm_text_utils import (
    clamp01 as _clamp01,
    decode_json_string_fragment as _decode_json_string_fragment,
    extract_compose_digest_fields as _extract_compose_digest_fields,
    extract_first_json_object as _extract_first_json_object,
    extract_json_string_value_loose as _extract_json_string_value_loose,
    normalize_url_for_match as _normalize_url_for_match,
    parse_json_string_array as _parse_json_string_array,
    strip_code_fence as _strip_code_fence,
    summary_max_tokens as _summary_max_tokens,
)
from app.services.llm_catalog import model_pricing, model_supports
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
from app.services.title_translation_common import TITLE_TRANSLATION_SCHEMA, run_title_translation
from app.services.digest_task_common import (
    build_cluster_draft_task,
    build_digest_task,
    build_simple_digest_input,
    parse_cluster_draft_result,
    parse_digest_result,
)
from app.services.feed_task_common import (
    build_ask_task,
    build_rank_feed_task,
    build_seed_sites_task,
    parse_ask_result,
    parse_rank_feed_result,
    parse_seed_sites_result,
)

_log = logging.getLogger(__name__)
_GROQ_PRICING_SOURCE_VERSION = "groq_static_2026_03"
_LEGACY_MODEL_PRICING = {
    "openai/gpt-oss-20b": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.30, "cache_read_per_mtok_usd": 0.0375},
    "openai/gpt-oss-120b": {"input_per_mtok_usd": 0.15, "output_per_mtok_usd": 0.60, "cache_read_per_mtok_usd": 0.075},
    "llama-3.1-8b-instant": {"input_per_mtok_usd": 0.05, "output_per_mtok_usd": 0.08, "cache_read_per_mtok_usd": 0.0},
    "llama-3.3-70b-versatile": {"input_per_mtok_usd": 0.59, "output_per_mtok_usd": 0.79, "cache_read_per_mtok_usd": 0.0},
    "meta-llama/llama-4-scout-17b-16e-instruct": {"input_per_mtok_usd": 0.11, "output_per_mtok_usd": 0.34, "cache_read_per_mtok_usd": 0.0},
    "qwen/qwen3-32b": {"input_per_mtok_usd": 0.29, "output_per_mtok_usd": 0.59, "cache_read_per_mtok_usd": 0.0},
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


def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _normalize_model_name(model: str) -> str:
    return str(model or "").strip()


def _normalize_model_family(model: str) -> str:
    m = _normalize_model_name(model)
    if model_pricing(m) is not None:
        return m
    for family in sorted(_LEGACY_MODEL_PRICING.keys(), key=len, reverse=True):
        if m == family or m.startswith(family + "-"):
            return family
    return m


def _pricing_for_model(model: str, purpose: str) -> dict:
    family = _normalize_model_family(model)
    catalog_pricing = model_pricing(family) or model_pricing(model)
    base = dict(catalog_pricing or _LEGACY_MODEL_PRICING.get(family, {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0}))
    source = str(base.get("pricing_source") or _GROQ_PRICING_SOURCE_VERSION)
    prefix = f"GROQ_{purpose.upper()}_"
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
    non_cached_input_tokens = max(0, int(usage.get("input_tokens", 0) or 0) - int(usage.get("cache_read_input_tokens", 0) or 0))
    total = 0.0
    total += non_cached_input_tokens / 1_000_000 * p["input_per_mtok_usd"]
    total += int(usage.get("output_tokens", 0) or 0) / 1_000_000 * p["output_per_mtok_usd"]
    total += int(usage.get("cache_read_input_tokens", 0) or 0) / 1_000_000 * p.get("cache_read_per_mtok_usd", 0.0)
    return round(total, 8)


def _llm_meta(model: str, purpose: str, usage: dict) -> dict:
    pricing = _pricing_for_model(model, purpose)
    actual_model = _normalize_model_name(model)
    return {
        "provider": "groq",
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", _GROQ_PRICING_SOURCE_VERSION),
        "input_tokens": int(usage.get("input_tokens", 0) or 0),
        "output_tokens": int(usage.get("output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }


def _supports_strict_schema(model: str) -> bool:
    family = _normalize_model_family(model)
    return model_supports(family, "supports_strict_json_schema") or model_supports(model, "supports_strict_json_schema")


def _usage_from_response(data: dict) -> dict:
    usage = data.get("usage") or {}
    prompt_details = usage.get("prompt_tokens_details") or {}
    return {
        "input_tokens": int(usage.get("prompt_tokens", 0) or 0),
        "output_tokens": int(usage.get("completion_tokens", 0) or 0),
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": int(prompt_details.get("cached_tokens", 0) or 0),
    }


def _chat_json(
    prompt: str,
    model: str,
    api_key: str,
    *,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    schema_name: str = "response",
    timeout_sec: float | None = None,
) -> tuple[str, dict]:
    api_key = (api_key or "").strip()
    if not api_key:
        raise RuntimeError("groq api key is required")
    body: dict = {
        "model": _normalize_model_name(model),
        "messages": [],
        "temperature": 0.2,
        "max_tokens": max_output_tokens,
    }
    if system_instruction:
        body["messages"].append({"role": "system", "content": system_instruction})
    body["messages"].append({"role": "user", "content": prompt})
    use_strict_schema = response_schema is not None and _supports_strict_schema(model)
    if response_schema is not None:
        if use_strict_schema:
            body["response_format"] = {
                "type": "json_schema",
                "json_schema": {
                    "name": schema_name,
                    "strict": True,
                    "schema": response_schema,
                },
            }
        else:
            body["response_format"] = {"type": "json_object"}
    url = "https://api.groq.com/openai/v1/chat/completions"
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("GROQ_TIMEOUT_SEC", 90.0)
    attempts = max(1, int(os.getenv("GROQ_RETRY_ATTEMPTS", "3") or "3"))
    base_sleep_sec = _env_timeout_seconds("GROQ_RETRY_BASE_SEC", 0.5)
    retryable_status = {408, 409, 429, 500, 502, 503, 504}

    def _is_json_validation_error(resp: httpx.Response) -> bool:
        return resp.status_code == 400 and "json_validate_failed" in (resp.text or "")

    resp: httpx.Response | None = None
    last_error: Exception | None = None
    with httpx.Client(timeout=req_timeout) as client:
        for i in range(attempts):
            try:
                resp = client.post(url, headers=headers, json=body)
                if _is_json_validation_error(resp) and response_schema is not None:
                    if use_strict_schema:
                        fallback_body = dict(body)
                        fallback_body["response_format"] = {"type": "json_object"}
                        resp = client.post(url, headers=headers, json=fallback_body)
                    if _is_json_validation_error(resp):
                        fallback_body = dict(body)
                        fallback_body.pop("response_format", None)
                        resp = client.post(url, headers=headers, json=fallback_body)
            except Exception as e:
                last_error = e
                if i < attempts - 1:
                    sleep_sec = base_sleep_sec * (2**i)
                    _log.warning("groq chat.completions request failed model=%s retry_in=%.1fs attempt=%d/%d err=%s", _normalize_model_name(model), sleep_sec, i + 1, attempts, e)
                    time.sleep(sleep_sec)
                    continue
                raise RuntimeError(f"groq chat.completions request failed: {e}") from e

            if resp.status_code < 400:
                break
            if resp.status_code in retryable_status and i < attempts - 1:
                sleep_sec = base_sleep_sec * (2**i)
                _log.warning("groq chat.completions retrying model=%s status=%d retry_in=%.1fs attempt=%d/%d", _normalize_model_name(model), resp.status_code, sleep_sec, i + 1, attempts)
                time.sleep(sleep_sec)
                continue
            break
    if resp is None:
        if last_error is not None:
            raise RuntimeError(f"groq chat.completions request failed: {last_error}") from last_error
        raise RuntimeError("groq chat.completions failed: no response")
    if resp.status_code >= 400:
        raise RuntimeError(f"groq chat.completions failed status={resp.status_code} body={resp.text[:1000]}")
    data = resp.json() if resp.content else {}
    choices = data.get("choices") or []
    if not choices:
        raise RuntimeError("groq chat.completions failed: empty choices")
    message = choices[0].get("message") or {}
    content = message.get("content")
    if isinstance(content, list):
        text = "\n".join(str(part.get("text") or "") for part in content if isinstance(part, dict))
    else:
        text = str(content or "")
    return text.strip(), _usage_from_response(data)


def _translate_title_to_ja(title: str, model: str, api_key: str) -> str:
    system_instruction = """# Role
あなたは見出し翻訳の専門家です。

# Task
英語やその他外国語のニュース記事タイトルを自然な日本語に翻訳してください。

# Rules
- 出力は必ず有効なJSONオブジェクト1つのみ
- translated_title が翻訳結果
- 既に日本語タイトルなら translated_title は空文字
- 固有名詞・製品名・企業名は必要に応じて原語を維持"""
    prompt = f"""# Output
{{
  \"translated_title\": \"日本語訳\"
}}

# Input
タイトル: {title}
"""
    plain_prompt = f"""# Input
次のタイトルが外国語なら自然な日本語に翻訳し、日本語なら空文字を返してください。
タイトル: {title}
"""
    return run_title_translation(
        title,
        structured_call=lambda: str(
            (_extract_first_json_object(
                _chat_json(
                    prompt,
                    model,
                    api_key,
                    system_instruction=system_instruction,
                    max_output_tokens=180,
                    response_schema=TITLE_TRANSLATION_SCHEMA,
                    schema_name="translated_title",
                )[0]
            ) or {}).get("translated_title")
            or ""
        ),
        plain_retry_call=lambda: _chat_json(plain_prompt, model, api_key, max_output_tokens=120)[0],
    )


def extract_facts(title: str | None, content: str, model: str, api_key: str) -> dict:
    system_instruction = """# Role
あなたは正確かつ客観的なニュース要約の専門家です。

# Task
提供される記事から重要な事実を8〜18個の箇条書きで抽出してください。

# Rules
- 出力は必ず [\"事実1\", \"事実2\", ...] のJSON形式の配列のみとしてください。
- 余計な挨拶や解説は一切不要です。
- 事実は客観的かつ具体的に記述してください。
- 記事が英語の場合も、出力は自然な日本語にしてください。
- 固有名詞は原文を尊重し、適宜英字を維持してください。"""
    prompt = f"""# Input
タイトル: {title or '（不明）'}

本文:
{content}
"""
    text, usage = _chat_json(prompt, model, api_key, system_instruction=system_instruction, max_output_tokens=1500)
    return {"facts": _parse_json_string_array(text), "llm": _llm_meta(model, "facts", usage)}


def summarize(title: str | None, facts: list[str], source_text_chars: int | None = None, model: str = "openai/gpt-oss-120b", api_key: str = "") -> dict:
    task = build_summary_task(title, facts, source_text_chars)
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        system_instruction=task["system_instruction"],
        max_output_tokens=_summary_max_tokens(task["target_chars"]),
        response_schema=task["schema"],
        schema_name="summary",
    )
    data = _extract_first_json_object(text) or {}
    topics = data.get("topics", []) if isinstance(data.get("topics"), list) else []
    return finalize_summary_result(
        title=title,
        summary_text=str(data.get("summary") or "").strip(),
        topics=topics,
        raw_score_breakdown=data.get("score_breakdown"),
        score_reason=str(data.get("score_reason") or "").strip(),
        translated_title=str(data.get("translated_title") or "").strip(),
        translate_func=lambda raw_title: _translate_title_to_ja(raw_title, model, api_key),
        llm=_llm_meta(model, "summary", usage),
        error_prefix="groq summarize parse failed",
        response_text=text,
    )


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
    return run_summary_faithfulness_check(
        lambda: (
            lambda text, usage: (text, _llm_meta(model, "faithfulness_check", usage))
        )(
            *_chat_json(
                summary_faithfulness_prompt(title, facts, summary),
                model,
                api_key,
                system_instruction=summary_faithfulness_system_instruction(),
                max_output_tokens=320,
                response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
                schema_name="summary_faithfulness",
            )
        ),
        retry_call=lambda: (
            lambda text, usage: (text, _llm_meta(model, "faithfulness_check", usage))
        )(
            *_chat_json(
                summary_faithfulness_retry_prompt(title, facts, summary),
                model,
                api_key,
                system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                max_output_tokens=120,
                response_schema=None,
            )
        ),
    )


def check_facts(title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
    return run_facts_check(
        lambda: (
            lambda text, usage: (text, _llm_meta(model, "facts_check", usage))
        )(
            *_chat_json(
                facts_check_prompt(title, content, facts),
                model,
                api_key,
                system_instruction=facts_check_system_instruction(),
                max_output_tokens=320,
                response_schema=FACTS_CHECK_SCHEMA,
                schema_name="facts_check",
            )
        ),
        retry_call=lambda: (
            lambda text, usage: (text, _llm_meta(model, "facts_check", usage))
        )(
            *_chat_json(
                facts_check_retry_prompt(title, content, facts),
                model,
                api_key,
                system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                max_output_tokens=120,
                response_schema=None,
            )
        ),
    )


def translate_title(title: str, model: str = "openai/gpt-oss-20b", api_key: str = "") -> dict:
    src = (title or "").strip()
    if not src:
        return {"translated_title": "", "llm": None}
    translated = _translate_title_to_ja(src, model, api_key)
    return {"translated_title": translated[:300], "llm": None}


def compose_digest(digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
        }
    task = build_digest_task(digest_date, len(items), build_simple_digest_input(items))
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        system_instruction=task["system_instruction"],
        max_output_tokens=8000,
        response_schema=task["schema"],
        schema_name="digest",
        timeout_sec=_env_timeout_seconds("GROQ_COMPOSE_DIGEST_TIMEOUT_SEC", 240.0),
    )
    subject, body = parse_digest_result(text, error_prefix="groq compose_digest parse failed")
    return {"subject": subject, "body": body, "llm": _llm_meta(model, "digest", usage)}


def ask_question(query: str, candidates: list[dict], model: str, api_key: str) -> dict:
    if not candidates:
        return {
            "answer": "該当する記事は見つかりませんでした。",
            "bullets": [],
            "citations": [],
            "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0}),
        }
    task = build_ask_task(query, candidates)
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        system_instruction=task["system_instruction"],
        max_output_tokens=3200,
        response_schema=task["schema"],
        schema_name="ask",
    )
    result = parse_ask_result(text, candidates, error_prefix="groq ask missing answer")
    return {**result, "llm": _llm_meta(model, "ask", usage)}


def compose_digest_cluster_draft(cluster_label: str, item_count: int, topics: list[str], source_lines: list[str], model: str, api_key: str) -> dict:
    task = build_cluster_draft_task(str(cluster_label or "話題").strip() or "話題", item_count, topics, source_lines)
    if not task["source_lines"]:
        return {"draft_summary": "", "llm": _llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}
    try:
        text, usage = _chat_json(
            task["prompt"],
            model,
            api_key,
            system_instruction=task["system_instruction"],
            max_output_tokens=900,
            response_schema=task["schema"],
            schema_name="digest_cluster_draft",
        )
    except Exception as exc:
        _log.warning("groq compose_digest_cluster_draft primary attempt failed: %s", exc)
        try:
            text, usage = _chat_json(task["fallback_prompt"], model, api_key, max_output_tokens=500, response_schema=None)
        except Exception as retry_exc:
            _log.warning("groq compose_digest_cluster_draft fallback failed: %s", retry_exc)
            return {"draft_summary": "\n".join(task["source_lines"][:5]), "llm": _llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}

    draft = parse_cluster_draft_result(text, task["source_lines"])
    return {"draft_summary": draft, "llm": _llm_meta(model, "digest_cluster_draft", usage)}


def rank_feed_suggestions(existing_sources: list[dict], preferred_topics: list[str], candidates: list[dict], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2800, response_schema=task["schema"], schema_name="rank_feed_suggestions")
    return {"items": parse_rank_feed_result(text, task["candidates"]), "llm": _llm_meta(model, "source_suggestion", usage)}


def suggest_feed_seed_sites(existing_sources: list[dict], preferred_topics: list[str], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
    task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="suggest_feed_seed_sites")
    return {"items": parse_seed_sites_result(text, task["existing_sources"]), "llm": _llm_meta(model, "source_suggestion", usage)}
