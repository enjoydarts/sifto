import json
import logging
import os
import time

import httpx

from app.services.digest_task_common import (
    build_cluster_draft_task,
    build_digest_task,
    build_simple_digest_input,
    fallback_cluster_draft_from_source_lines,
    parse_cluster_draft_result,
    parse_digest_result,
)
from app.services.facts_check_common import (
    FACTS_CHECK_SCHEMA,
    facts_check_prompt,
    facts_check_retry_prompt,
    facts_check_system_instruction,
)
from app.services.facts_check_runner import run_facts_check
from app.services.facts_task_common import build_facts_localization_task, build_facts_task, parse_facts_result
from app.services.feed_task_common import (
    build_audio_briefing_script_task,
    build_ask_task,
    build_ask_navigator_task,
    build_briefing_navigator_task,
    build_item_navigator_task,
    build_source_navigator_task,
    build_rank_feed_task,
    build_seed_sites_rescue_prompt,
    build_seed_sites_task,
    merge_llm_usage,
    parse_audio_briefing_script_result,
    parse_ask_result,
    parse_ask_navigator_result,
    parse_briefing_navigator_result,
    parse_item_navigator_result,
    parse_source_navigator_result,
    parse_rank_feed_result,
    parse_seed_sites_result,
)
from app.services.llm_catalog import model_pricing, model_supports, resolve_model_id
from app.services.llm_text_utils import (
    audio_briefing_script_max_tokens as _audio_briefing_script_max_tokens,
    extract_first_json_object as _extract_first_json_object,
    facts_need_japanese_localization as _facts_need_japanese_localization,
    summary_max_tokens as _summary_max_tokens,
)
from app.services.openai_compat_transport import run_chat_json
from app.services.openrouter_service import _repair_structured_json_text
from app.services.summary_faithfulness_common import (
    SUMMARY_FAITHFULNESS_SCHEMA,
    summary_faithfulness_prompt,
    summary_faithfulness_retry_prompt,
    summary_faithfulness_system_instruction,
)
from app.services.summary_faithfulness_runner import run_summary_faithfulness_check
from app.services.summary_parse_common import finalize_summary_result
from app.services.summary_task_common import build_summary_task
from app.services.task_transport_common import with_execution_failures, wrap_json_transport
from app.services.title_translation_common import TITLE_TRANSLATION_SCHEMA, run_title_translation

_log = logging.getLogger(__name__)
_POE_PRICING_SOURCE_VERSION = "poe_snapshot"


def _env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        value = float(raw)
        return value if value > 0 else default
    except Exception:
        return default


def _normalize_model_name(model: str) -> str:
    return resolve_model_id(model)


def _supports_anthropic_transport(model: str) -> bool:
    resolved = _normalize_model_name(model).lower()
    return resolved.startswith("claude") or resolved.startswith("anthropic/claude")


def _pricing_for_model(model: str, purpose: str) -> dict:
    base = dict(model_pricing(model) or model_pricing(_normalize_model_name(model)) or {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0})
    source = str(base.get("pricing_source") or _POE_PRICING_SOURCE_VERSION)
    prefix = f"POE_{purpose.upper()}_"
    override_keys = (
        "INPUT_PER_MTOK_USD",
        "OUTPUT_PER_MTOK_USD",
        "CACHE_READ_PER_MTOK_USD",
    )
    for env_key in override_keys:
        raw = os.getenv(prefix + env_key)
        if raw is None or raw == "":
            continue
        try:
            value = float(raw)
        except Exception:
            continue
        field = env_key.lower()
        base[field] = value
        source = "env_override"
    base["pricing_source"] = source
    base["pricing_model_family"] = _normalize_model_name(model)
    return base


def _estimate_cost_usd(model: str, purpose: str, usage: dict) -> float:
    pricing = _pricing_for_model(model, purpose)
    non_cached_input_tokens = max(0, int(usage.get("input_tokens", 0) or 0) - int(usage.get("cache_read_input_tokens", 0) or 0))
    total = 0.0
    total += non_cached_input_tokens / 1_000_000 * float(pricing.get("input_per_mtok_usd", 0.0) or 0.0)
    total += int(usage.get("output_tokens", 0) or 0) / 1_000_000 * float(pricing.get("output_per_mtok_usd", 0.0) or 0.0)
    total += int(usage.get("cache_read_input_tokens", 0) or 0) / 1_000_000 * float(pricing.get("cache_read_per_mtok_usd", 0.0) or 0.0)
    return round(total, 8)


def _llm_meta(model: str, purpose: str, usage: dict) -> dict:
    requested_model = str(usage.get("requested_model") or model or "").strip()
    resolved_model = str(usage.get("resolved_model") or "").strip()
    pricing_target = resolved_model or requested_model or model
    pricing = _pricing_for_model(pricing_target, purpose)
    actual_model = _normalize_model_name(pricing_target)
    return with_execution_failures(
        {
            "provider": "poe",
            "model": requested_model or model,
            "requested_model": requested_model or model,
            "resolved_model": resolved_model,
            "pricing_model_family": pricing.get("pricing_model_family", actual_model),
            "pricing_source": pricing.get("pricing_source", _POE_PRICING_SOURCE_VERSION),
            "input_tokens": int(usage.get("input_tokens", 0) or 0),
            "output_tokens": int(usage.get("output_tokens", 0) or 0),
            "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
            "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
            "estimated_cost_usd": float(usage.get("billed_cost_usd")) if usage.get("billed_cost_usd") is not None else _estimate_cost_usd(actual_model, purpose, usage),
        },
        usage.get("execution_failures"),
    )


def _supports_strict_schema(model: str) -> bool:
    return model_supports(_normalize_model_name(model), "supports_strict_json_schema") or model_supports(model, "supports_strict_json_schema")


def _chat_json_openai_compat(
    prompt: str,
    model: str,
    api_key: str,
    *,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    schema_name: str = "response",
    timeout_sec: float | None = None,
    temperature: float | None = None,
    top_p: float | None = None,
) -> tuple[str, dict]:
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("POE_TIMEOUT_SEC", 90.0)
    attempts = max(1, int(os.getenv("POE_RETRY_ATTEMPTS", "3") or "3"))
    base_sleep_sec = _env_timeout_seconds("POE_RETRY_BASE_SEC", 0.5)
    text, usage = run_chat_json(
        prompt,
        model,
        api_key,
        url=os.getenv("POE_OPENAI_API_BASE_URL", "https://api.poe.com/v1/chat/completions"),
        normalize_model_name=_normalize_model_name,
        supports_strict_schema=_supports_strict_schema,
        timeout_sec=req_timeout,
        attempts=attempts,
        base_sleep_sec=base_sleep_sec,
        provider_name="poe",
        logger=_log,
        system_instruction=system_instruction,
        max_output_tokens=max_output_tokens,
        response_schema=response_schema,
        schema_name=schema_name,
        include_temperature=True,
        temperature=temperature,
        top_p=top_p,
    )
    return _repair_structured_json_text(text, model, response_schema), usage


def _anthropic_usage_from_response(data: dict) -> dict:
    usage = data.get("usage") or {}
    return {
        "input_tokens": int(usage.get("input_tokens", 0) or 0),
        "output_tokens": int(usage.get("output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
        "resolved_model": str(data.get("model") or "").strip() or None,
    }


def _chat_json_anthropic_compat(
    prompt: str,
    model: str,
    api_key: str,
    *,
    system_instruction: str | None = None,
    max_output_tokens: int = 1200,
    response_schema: dict | None = None,
    timeout_sec: float | None = None,
    temperature: float | None = None,
    top_p: float | None = None,
) -> tuple[str, dict]:
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("POE_TIMEOUT_SEC", 90.0)
    attempts = max(1, int(os.getenv("POE_RETRY_ATTEMPTS", "3") or "3"))
    base_sleep_sec = _env_timeout_seconds("POE_RETRY_BASE_SEC", 0.5)
    body: dict = {
        "model": _normalize_model_name(model),
        "max_tokens": max_output_tokens,
        "messages": [{"role": "user", "content": prompt}],
    }
    if system_instruction:
        body["system"] = system_instruction
    if temperature is not None:
        body["temperature"] = temperature
    if top_p is not None:
        body["top_p"] = top_p
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
        "anthropic-version": os.getenv("POE_ANTHROPIC_VERSION", "2023-06-01"),
    }
    retryable_status = {408, 409, 429, 500, 502, 503, 504}
    retry_usage: dict = {}
    requested_model = str(model or "").strip() or None
    resp: httpx.Response | None = None
    last_error: Exception | None = None

    for attempt in range(attempts):
        try:
            with httpx.Client(timeout=req_timeout) as client:
                resp = client.post(os.getenv("POE_ANTHROPIC_API_BASE_URL", "https://api.poe.com/v1/messages"), headers=headers, json=body)
        except Exception as err:
            last_error = err
            if attempt < attempts - 1:
                with_execution_failures(retry_usage, [{"model": requested_model, "reason": f"request failed: {err}"}] if requested_model else None)
                time.sleep(base_sleep_sec * (2**attempt))
                continue
            raise RuntimeError(f"poe anthropic messages request failed: {err}") from err

        if resp.status_code < 400:
            break
        if resp.status_code in retryable_status and attempt < attempts - 1:
            with_execution_failures(retry_usage, [{"model": requested_model, "reason": f"status={resp.status_code} body={resp.text[:1000]}"}] if requested_model else None)
            time.sleep(base_sleep_sec * (2**attempt))
            continue
        break

    if resp is None:
        if last_error is not None:
            raise RuntimeError(f"poe anthropic messages request failed: {last_error}") from last_error
        raise RuntimeError("poe anthropic messages failed: no response")
    if resp.status_code >= 400:
        raise RuntimeError(f"poe anthropic messages failed status={resp.status_code} body={resp.text[:1000]}")

    data = resp.json() if resp.content else {}
    content = data.get("content") or []
    parts = []
    for item in content:
        if isinstance(item, dict) and str(item.get("type") or "") == "text":
            parts.append(str(item.get("text") or ""))
    text = "\n".join(parts).strip()
    usage = _anthropic_usage_from_response(data)
    usage["requested_model"] = requested_model
    if retry_usage.get("execution_failures"):
        usage["execution_failures"] = list(retry_usage["execution_failures"])
    return _repair_structured_json_text(text, model, response_schema), usage


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
    temperature: float | None = None,
    top_p: float | None = None,
) -> tuple[str, dict]:
    api_key = (api_key or "").strip()
    if not api_key:
        raise RuntimeError("poe api key is required")
    if _supports_anthropic_transport(model):
        return _chat_json_anthropic_compat(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=response_schema,
            timeout_sec=timeout_sec,
            temperature=temperature,
            top_p=top_p,
        )
    return _chat_json_openai_compat(
        prompt,
        model,
        api_key,
        system_instruction=system_instruction,
        max_output_tokens=max_output_tokens,
        response_schema=response_schema,
        schema_name=schema_name,
        timeout_sec=timeout_sec,
        temperature=temperature,
        top_p=top_p,
    )


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
    task = build_facts_task(title, content, output_mode="object")
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        system_instruction=task["system_instruction"],
        max_output_tokens=1500,
        response_schema=task["schema"],
        schema_name="facts",
    )
    facts = parse_facts_result(text)
    localization_llm = None
    if not facts:
        raise RuntimeError(f"poe extract_facts parse failed: response_snippet={text[:500]}")
    if _facts_need_japanese_localization(facts):
        localize_task = build_facts_localization_task(title, facts)
        localized_text, localized_usage = _chat_json(
            localize_task["prompt"],
            model,
            api_key,
            system_instruction=localize_task["system_instruction"],
            max_output_tokens=1200,
            response_schema=localize_task["schema"],
            schema_name="facts_localized",
        )
        localized_facts = parse_facts_result(localized_text)
        if localized_facts:
            facts = localized_facts
            localization_llm = _llm_meta(model, "facts_localization", localized_usage)
    return {"facts": facts, "llm": _llm_meta(model, "facts", usage), "facts_localization_llm": localization_llm}


def summarize(title: str | None, facts: list[str], source_text_chars: int | None = None, model: str = "poe::Claude-Sonnet-4.5", api_key: str = "") -> dict:
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
        error_prefix="poe summarize parse failed",
        response_text=text,
    )


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
    return run_summary_faithfulness_check(
        lambda: wrap_json_transport(
            lambda: _chat_json(
                summary_faithfulness_prompt(title, facts, summary),
                model,
                api_key,
                system_instruction=summary_faithfulness_system_instruction(),
                max_output_tokens=320,
                response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
                schema_name="summary_faithfulness",
            ),
            lambda usage: _llm_meta(model, "faithfulness_check", usage),
        ),
        retry_call=lambda: wrap_json_transport(
            lambda: _chat_json(
                summary_faithfulness_retry_prompt(title, facts, summary),
                model,
                api_key,
                system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                max_output_tokens=120,
                response_schema=None,
            ),
            lambda usage: _llm_meta(model, "faithfulness_check", usage),
        ),
    )


def check_facts(title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
    return run_facts_check(
        lambda: wrap_json_transport(
            lambda: _chat_json(
                facts_check_prompt(title, content, facts),
                model,
                api_key,
                system_instruction=facts_check_system_instruction(),
                max_output_tokens=320,
                response_schema=FACTS_CHECK_SCHEMA,
                schema_name="facts_check",
            ),
            lambda usage: _llm_meta(model, "facts_check", usage),
        ),
        retry_call=lambda: wrap_json_transport(
            lambda: _chat_json(
                facts_check_retry_prompt(title, content, facts),
                model,
                api_key,
                system_instruction=facts_check_system_instruction(),
                max_output_tokens=220,
                response_schema=FACTS_CHECK_SCHEMA,
                schema_name="facts_check",
            ),
            lambda usage: _llm_meta(model, "facts_check", usage),
        ),
    )


def translate_title(title: str, model: str = "poe::Claude-Sonnet-4.5", api_key: str = "") -> dict:
    src = (title or "").strip()
    if not src:
        return {"translated_title": "", "llm": None}
    return {"translated_title": _translate_title_to_ja(src, model, api_key)[:300], "llm": None}


def compose_digest(digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
    if not items:
        return {"subject": f"Sifto Digest - {digest_date}", "body": "本日のダイジェスト対象記事はありませんでした。", "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0})}
    task = build_digest_task(digest_date, len(items), build_simple_digest_input(items))
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        system_instruction=task["system_instruction"],
        max_output_tokens=8000,
        response_schema=task["schema"],
        schema_name="digest",
        timeout_sec=_env_timeout_seconds("POE_COMPOSE_DIGEST_TIMEOUT_SEC", 240.0),
    )
    subject, body = parse_digest_result(text, error_prefix="poe compose_digest parse failed")
    return {"subject": subject, "body": body, "llm": _llm_meta(model, "digest", usage)}


def ask_question(query: str, candidates: list[dict], model: str, api_key: str) -> dict:
    if not candidates:
        return {"answer": "該当する記事は見つかりませんでした。", "bullets": [], "citations": [], "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0})}
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
    result = parse_ask_result(text, candidates, error_prefix="poe ask missing answer")
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
            max_output_tokens=1500,
            response_schema=task["schema"],
            schema_name="digest_cluster_draft",
        )
    except Exception as exc:
        _log.warning("poe compose_digest_cluster_draft primary attempt failed: %s", exc)
        try:
            text, usage = _chat_json(task["fallback_prompt"], model, api_key, max_output_tokens=1500, response_schema=None)
        except Exception as retry_exc:
            _log.warning("poe compose_digest_cluster_draft fallback failed: %s", retry_exc)
            return {"draft_summary": fallback_cluster_draft_from_source_lines(task["source_lines"]), "llm": _llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}
    draft = parse_cluster_draft_result(text, task["source_lines"])
    return {"draft_summary": draft, "llm": _llm_meta(model, "digest_cluster_draft", usage)}


def rank_feed_suggestions(existing_sources: list[dict], preferred_topics: list[str], candidates: list[dict], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2800, response_schema=task["schema"], schema_name="rank_feed_suggestions")
    return {"items": parse_rank_feed_result(text, task["candidates"]), "llm": _llm_meta(model, "source_suggestion", usage)}


def generate_briefing_navigator(persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
    task = build_briefing_navigator_task(persona, candidates, intro_context)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=1800, response_schema=task["schema"], schema_name="briefing_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
    out = parse_briefing_navigator_result(text, task["candidates"])
    return {"intro": out["intro"], "picks": out["picks"], "llm": _llm_meta(model, "briefing_navigator", usage)}


def generate_item_navigator(persona: str, article: dict, model: str, api_key: str) -> dict:
    task = build_item_navigator_task(persona, article)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="item_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
    out = parse_item_navigator_result(text, task["article"])
    return {"headline": out["headline"], "commentary": out["commentary"], "stance_tags": out["stance_tags"], "llm": _llm_meta(model, "item_navigator", usage)}


def generate_audio_briefing_script(
    persona: str,
    articles: list[dict],
    intro_context: dict,
    target_duration_minutes: int,
    target_chars: int,
    chars_per_minute: int,
    include_opening: bool,
    include_overall_summary: bool,
    include_article_segments: bool,
    include_ending: bool,
    model: str,
    api_key: str,
) -> dict:
    task = build_audio_briefing_script_task(
        persona,
        articles,
        intro_context,
        target_duration_minutes=target_duration_minutes,
        target_chars=target_chars,
        chars_per_minute=chars_per_minute,
        include_opening=include_opening,
        include_overall_summary=include_overall_summary,
        include_article_segments=include_article_segments,
        include_ending=include_ending,
    )
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=_audio_briefing_script_max_tokens(task["target_chars"]), response_schema=task["schema"], schema_name="audio_briefing_script")
    out = parse_audio_briefing_script_result(
        text,
        task["articles"],
        persona,
        target_chars=target_chars,
        include_opening=include_opening,
        include_overall_summary=include_overall_summary,
        include_article_segments=include_article_segments,
        include_ending=include_ending,
    )
    return {
        "opening": out["opening"],
        "overall_summary": out["overall_summary"],
        "article_segments": out["article_segments"],
        "ending": out["ending"],
        "llm": _llm_meta(model, "audio_briefing_script", usage),
    }


def generate_ask_navigator(persona: str, ask_input: dict, model: str, api_key: str) -> dict:
    task = build_ask_navigator_task(persona, ask_input)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2400, response_schema=task["schema"], schema_name="ask_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
    out = parse_ask_navigator_result(text, task["input"])
    return {"headline": out["headline"], "commentary": out["commentary"], "next_angles": out["next_angles"], "llm": _llm_meta(model, "ask_navigator", usage)}


def generate_source_navigator(persona: str, candidates: list[dict], model: str, api_key: str) -> dict:
    task = build_source_navigator_task(persona, candidates)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2600, response_schema=task["schema"], schema_name="source_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
    out = parse_source_navigator_result(text, task["candidates"])
    return {"overview": out["overview"], "keep": out["keep"], "watch": out["watch"], "standout": out["standout"], "llm": _llm_meta(model, "source_navigator", usage)}


def suggest_feed_seed_sites(existing_sources: list[dict], preferred_topics: list[str], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
    task = build_seed_sites_task(existing_sources, preferred_topics=preferred_topics, positive_examples=positive_examples, negative_examples=negative_examples)
    text, usage = _chat_json(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="suggest_feed_seed_sites")
    out = parse_seed_sites_result(text, task["existing_sources"])
    if len(out) == 0:
        try:
            rescue_text, rescue_usage = _chat_json(
                build_seed_sites_rescue_prompt(task["existing_sources"], task["preferred_topics"]),
                model,
                api_key,
                max_output_tokens=1800,
                response_schema=task["schema"],
                schema_name="suggest_feed_seed_sites",
            )
            out.extend(parse_seed_sites_result(rescue_text, task["existing_sources"]))
            usage = merge_llm_usage(usage, rescue_usage)
        except Exception as exc:
            _log.warning("poe suggest_feed_seed_sites rescue failed: %s", exc)
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}
