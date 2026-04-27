import asyncio
import json
import hashlib
import logging
import os
import re
from urllib.parse import urlparse
from app.services.llm_catalog import model_pricing
from app.services.gemini_transport import (
    audio_briefing_script_context_cache_enabled as _audio_briefing_script_context_cache_enabled,
    cache_key_hash as _cache_key_hash,
    env_int as _env_int,
    env_timeout_seconds as _env_timeout_seconds,
    generate_content as _gemini_generate_content,
    generate_content_async as _gemini_generate_content_async,
    summary_context_cache_enabled as _summary_context_cache_enabled,
)
from app.services.llm_text_utils import (
    audio_briefing_script_max_tokens as _audio_briefing_script_max_tokens,
    clamp01 as _clamp01,
    clamp_int as _clamp_int,
    decode_json_string_fragment as _decode_json_string_fragment,
    extract_compose_digest_fields as _extract_compose_digest_fields,
    extract_first_json_object as _extract_first_json_object,
    extract_json_string_value_loose as _extract_json_string_value_loose,
    facts_need_japanese_localization as _facts_need_japanese_localization,
    normalize_url_for_match as _normalize_url_for_match,
    parse_json_string_array as _parse_json_string_array,
    strip_code_fence as _strip_code_fence,
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
from app.services.title_translation_common import TITLE_TRANSLATION_SCHEMA, run_title_translation
from app.services.digest_task_common import (
    DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
    build_cluster_draft_task,
    build_digest_task,
    parse_cluster_draft_result,
    parse_digest_result,
)
from app.services.feed_task_common import (
    ASK_MAX_OUTPUT_TOKENS,
    build_ai_navigator_brief_task,
    build_audio_briefing_script_task,
    build_ask_rerank_task,
    build_ask_task,
    build_ask_navigator_task,
    build_briefing_navigator_task,
    build_item_navigator_task,
    build_source_navigator_task,
    build_rank_feed_task,
    build_seed_sites_task,
    parse_ai_navigator_brief_result,
    parse_audio_briefing_script_result,
    parse_ask_rerank_result,
    parse_ask_result,
    parse_ask_navigator_result,
    parse_briefing_navigator_result,
    parse_item_navigator_result,
    parse_source_navigator_result,
    parse_rank_feed_result,
    parse_seed_sites_result,
)
from app.services.facts_task_common import build_facts_localization_task, build_facts_task, parse_facts_result
from app.services.task_transport_common import with_execution_failures, wrap_usage_transport

_log = logging.getLogger(__name__)
_GEMINI_PRICING_SOURCE_VERSION = "google_aistudio_static_2026_02"

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


def _generate_content(*args, **kwargs):
    return _gemini_generate_content(*args, normalize_model_name=_normalize_model_name, logger=_log, **kwargs)


async def _generate_content_async(*args, **kwargs):
    return await _gemini_generate_content_async(*args, normalize_model_name=_normalize_model_name, logger=_log, **kwargs)

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
    return with_execution_failures({
        "provider": "google",
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", _GEMINI_PRICING_SOURCE_VERSION),
        "input_tokens": usage.get("input_tokens", 0),
        "output_tokens": usage.get("output_tokens", 0),
        "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
        "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }, usage.get("execution_failures"))


def _translate_title_to_ja(title: str, model: str, api_key: str) -> str:
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
        structured_call=lambda: str(
            (_extract_first_json_object(
                _generate_content(
                    prompt,
                    model=model,
                    api_key=api_key,
                    max_output_tokens=200,
                    response_schema=TITLE_TRANSLATION_SCHEMA,
                )[0]
            ) or {}).get("translated_title")
            or ""
        ),
        plain_retry_call=lambda: _generate_content(
            plain_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=120,
            response_schema=None,
        )[0],
    )


def rank_feed_suggestions(
    existing_sources: list[dict],
    preferred_topics: list[str],
    candidates: list[dict],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
    model: str,
    api_key: str,
) -> dict:
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2800,
        response_schema=task["schema"],
    )
    out = parse_rank_feed_result(text, task["candidates"])
    # Rescue: Gemini が空配列を返した場合は簡易再プロンプトで再取得する。
    if len(out) == 0 and len(task["candidates"]) > 0:
        rescue_prompt = f"""候補フィードを優先度順に再提示してください。必ず最低10件は返してください。
JSONのみ:
{{
  "items":[{{"id":"c001","reason":"短い理由","confidence":0.0-1.0}}]
}}

興味トピック:
{json.dumps(task["preferred_topics"], ensure_ascii=False)}

候補フィード:
{json.dumps(task["candidates"], ensure_ascii=False)}
"""
        rescue_text, rescue_usage = _generate_content(
            rescue_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=1800,
            response_schema=task["schema"],
        )
        out.extend(parse_rank_feed_result(rescue_text, task["candidates"]))
        usage["input_tokens"] = int(usage.get("input_tokens", 0)) + int(rescue_usage.get("input_tokens", 0))
        usage["output_tokens"] = int(usage.get("output_tokens", 0)) + int(rescue_usage.get("output_tokens", 0))
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


def generate_briefing_navigator(persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
    task = build_briefing_navigator_task(persona, candidates, intro_context)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=1800,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_briefing_navigator_result(text, task["candidates"])
    return {"intro": out["intro"], "picks": out["picks"], "llm": _llm_meta(model, "briefing_navigator", usage)}


def compose_ai_navigator_brief(persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
    task = build_ai_navigator_brief_task(persona, candidates, intro_context)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=3200,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_ai_navigator_brief_result(text, task["candidates"], intro_context)
    return {"title": out["title"], "intro": out["intro"], "summary": out["summary"], "ending": out["ending"], "items": out["items"], "llm": _llm_meta(model, "ai_navigator_brief", usage)}


def generate_item_navigator(persona: str, article: dict, model: str, api_key: str) -> dict:
    task = build_item_navigator_task(persona, article)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2200,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
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
    api_key_hash = hashlib.sha256((api_key or "").encode("utf-8")).hexdigest()[:16]
    cache_key = None
    if _audio_briefing_script_context_cache_enabled():
        cache_key = _cache_key_hash(
            [
                _normalize_model_name(model),
                "audio-briefing-script-v1",
                api_key_hash,
                task["system_instruction"],
            ]
        )
    text, usage = _generate_content(
        task["user_prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=_audio_briefing_script_max_tokens(task["target_chars"], str((intro_context or {}).get("audio_briefing_conversation_mode") or "single")),
        system_instruction=task["system_instruction"],
        context_cache_key=cache_key,
        response_schema=task["schema"],
    )
    out = parse_audio_briefing_script_result(
        text,
        task["articles"],
        persona,
        conversation_mode=str((intro_context or {}).get("audio_briefing_conversation_mode") or "single"),
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
        "turns": out["turns"],
        "ending": out["ending"],
        "llm": _llm_meta(model, "audio_briefing_script", usage),
    }


def generate_ask_navigator(persona: str, ask_input: dict, model: str, api_key: str) -> dict:
    task = build_ask_navigator_task(persona, ask_input)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2400,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_ask_navigator_result(text, task["input"])
    return {"headline": out["headline"], "commentary": out["commentary"], "next_angles": out["next_angles"], "llm": _llm_meta(model, "ask_navigator", usage)}


def generate_source_navigator(persona: str, candidates: list[dict], model: str, api_key: str) -> dict:
    task = build_source_navigator_task(persona, candidates)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2600,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_source_navigator_result(text, task["candidates"])
    return {"overview": out["overview"], "keep": out["keep"], "watch": out["watch"], "standout": out["standout"], "llm": _llm_meta(model, "source_navigator", usage)}


def suggest_feed_seed_sites(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
    model: str,
    api_key: str,
) -> dict:
    task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2200,
        response_schema=task["schema"],
    )
    out = parse_seed_sites_result(text, task["existing_sources"])
    if len(out) == 0:
        rescue_prompt = f"""既存ソースと重複しないサイトURL候補を必ず10件以上返してください。JSONのみ。
{{
  "items": [
    {{"url":"https://...", "reason":"..."}}
  ]
}}
既存ソース:
{json.dumps(task["existing_sources"], ensure_ascii=False)}
興味トピック:
{json.dumps(task["preferred_topics"], ensure_ascii=False)}
"""
        rescue_text, rescue_usage = _generate_content(
            rescue_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=1800,
            response_schema=task["schema"],
        )
        out.extend(parse_seed_sites_result(rescue_text, task["existing_sources"]))
        usage["input_tokens"] = int(usage.get("input_tokens", 0)) + int(rescue_usage.get("input_tokens", 0))
        usage["output_tokens"] = int(usage.get("output_tokens", 0)) + int(rescue_usage.get("output_tokens", 0))
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


def extract_facts(title: str | None, content: str, model: str, api_key: str) -> dict:
    task = build_facts_task(title, content, output_mode="array")
    text, usage = _generate_content(task["prompt"], model=model, api_key=api_key, max_output_tokens=1500, system_instruction=task["system_instruction"])
    facts = parse_facts_result(text)
    localization_llm = None
    if not facts:
        raise RuntimeError(f"gemini extract_facts parse failed: response_snippet={text[:500]}")
    if _facts_need_japanese_localization(facts):
        localize_task = build_facts_localization_task(title, facts)
        localized_text, localized_usage = _generate_content(
            localize_task["prompt"],
            model=model,
            api_key=api_key,
            max_output_tokens=1200,
            system_instruction=localize_task["system_instruction"],
            response_schema=localize_task["schema"],
        )
        localized_facts = parse_facts_result(localized_text)
        if localized_facts:
            facts = localized_facts
            localization_llm = _llm_meta(model, "facts_localization", localized_usage)
    return {"facts": facts, "llm": _llm_meta(model, "facts", usage), "facts_localization_llm": localization_llm}


def summarize(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    model: str = "gemini-2.5-flash",
    api_key: str = "",
) -> dict:
    task = build_summary_task(title, facts, source_text_chars)
    max_tokens = _summary_max_tokens(task["target_chars"])
    api_key_hash = hashlib.sha256((api_key or "").encode("utf-8")).hexdigest()[:16]
    cache_key = None
    if _summary_context_cache_enabled():
        cache_key = _cache_key_hash([_normalize_model_name(model), "summary-v2", api_key_hash, task["system_instruction"]])
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=max_tokens,
        system_instruction=task["system_instruction"],
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
    return finalize_summary_result(
        title=title,
        summary_text=str(data.get("summary", "")).strip(),
        topics=topics,
        genre=str(data.get("genre") or "").strip(),
        other_label=str(data.get("other_label") or "").strip(),
        raw_score_breakdown=data.get("score_breakdown") if isinstance(data.get("score_breakdown"), dict) else {},
        score_reason=str(data.get("score_reason") or "").strip(),
        translated_title=str(data.get("translated_title") or "").strip(),
        translate_func=lambda raw_title: _translate_title_to_ja(raw_title, model=model, api_key=api_key),
        llm=_llm_meta(model, "summary", usage),
        error_prefix="gemini summarize parse failed",
        response_text=text,
    )


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
    return run_summary_faithfulness_check(
        lambda: wrap_usage_transport(
            lambda: _generate_content(
                summary_faithfulness_prompt(title, facts, summary),
                model=model,
                api_key=api_key,
                max_output_tokens=320,
                system_instruction=summary_faithfulness_system_instruction(),
                response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
            ),
            lambda usage: _llm_meta(model, "faithfulness_check", usage),
        ),
        retry_call=lambda: wrap_usage_transport(
            lambda: _generate_content(
                summary_faithfulness_retry_prompt(title, facts, summary),
                model=model,
                api_key=api_key,
                max_output_tokens=120,
                system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                response_schema=None,
                response_mime_type="text/plain",
            ),
            lambda usage: _llm_meta(model, "faithfulness_check", usage),
        ),
    )


def check_facts(title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
    return run_facts_check(
        lambda: wrap_usage_transport(
            lambda: _generate_content(
                facts_check_prompt(title, content, facts),
                model=model,
                api_key=api_key,
                max_output_tokens=320,
                system_instruction=facts_check_system_instruction(),
                response_schema=FACTS_CHECK_SCHEMA,
            ),
            lambda usage: _llm_meta(model, "facts_check", usage),
        ),
        retry_call=lambda: wrap_usage_transport(
            lambda: _generate_content(
                facts_check_retry_prompt(title, content, facts),
                model=model,
                api_key=api_key,
                max_output_tokens=220,
                system_instruction=facts_check_system_instruction(),
                response_schema=FACTS_CHECK_SCHEMA,
            ),
            lambda usage: _llm_meta(model, "facts_check", usage),
        ),
    )


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
    task = build_digest_task(digest_date, len(items), digest_input, input_mode=input_mode)

    compose_timeout = _env_timeout_seconds("GEMINI_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0)
    last_text = ""
    last_error = "unknown"
    for max_tokens in (10000, 15000):
        text, usage = _generate_content(
            task["prompt"],
            model=model,
            api_key=api_key,
            max_output_tokens=max_tokens,
            response_schema=task["schema"],
            timeout_sec=compose_timeout,
            system_instruction=task["system_instruction"],
        )
        last_text = text
        try:
            subject, body = parse_digest_result(text, error_prefix="gemini compose_digest parse failed")
        except Exception:
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
            "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0}),
        }
    task = build_ask_task(query, candidates)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=ASK_MAX_OUTPUT_TOKENS,
        response_schema=task["schema"],
        timeout_sec=_env_timeout_seconds("GEMINI_TIMEOUT_SEC", 300.0),
        system_instruction=task["system_instruction"],
    )
    result = parse_ask_result(text, candidates, error_prefix="gemini ask missing answer")
    return {**result, "llm": _llm_meta(model, "ask", usage)}


def ask_rerank(query: str, candidates: list[dict], top_k: int, model: str, api_key: str) -> dict:
    if not candidates:
        return {"items": [], "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0})}
    task = build_ask_rerank_task(query, candidates, top_k)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=1600,
        response_schema=task["schema"],
        timeout_sec=_env_timeout_seconds("GEMINI_TIMEOUT_SEC", 300.0),
    )
    result = parse_ask_rerank_result(text, candidates, task["top_k"])
    return {**result, "llm": _llm_meta(model, "ask", usage)}


def compose_digest_cluster_draft(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    model: str,
    api_key: str,
) -> dict:
    task = build_cluster_draft_task(cluster_label, item_count, topics, source_lines)
    text, usage = _generate_content(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
        system_instruction=task["system_instruction"],
        response_schema=task["schema"],
    )
    summary = parse_cluster_draft_result(text, task["source_lines"])
    return {
        "draft_summary": summary,
        "llm": _llm_meta(model, "digest_cluster_draft", usage),
    }


async def rank_feed_suggestions_async(
    existing_sources: list[dict],
    preferred_topics: list[str],
    candidates: list[dict],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
    model: str,
    api_key: str,
) -> dict:
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2800,
        response_schema=task["schema"],
    )
    out = parse_rank_feed_result(text, task["candidates"])
    if len(out) == 0 and len(task["candidates"]) > 0:
        rescue_prompt = f"""候補フィードを優先度順に再提示してください。必ず最低10件は返してください。
JSONのみ:
{{
  "items":[{{"id":"c001","reason":"短い理由","confidence":0.0-1.0}}]
}}

興味トピック:
{json.dumps(task["preferred_topics"], ensure_ascii=False)}

候補フィード:
{json.dumps(task["candidates"], ensure_ascii=False)}
"""
        rescue_text, rescue_usage = await _generate_content_async(
            rescue_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=1800,
            response_schema=task["schema"],
        )
        out.extend(parse_rank_feed_result(rescue_text, task["candidates"]))
        usage["input_tokens"] = int(usage.get("input_tokens", 0)) + int(rescue_usage.get("input_tokens", 0))
        usage["output_tokens"] = int(usage.get("output_tokens", 0)) + int(rescue_usage.get("output_tokens", 0))
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


async def generate_briefing_navigator_async(persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
    task = build_briefing_navigator_task(persona, candidates, intro_context)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=1800,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_briefing_navigator_result(text, task["candidates"])
    return {"intro": out["intro"], "picks": out["picks"], "llm": _llm_meta(model, "briefing_navigator", usage)}


async def compose_ai_navigator_brief_async(persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
    task = build_ai_navigator_brief_task(persona, candidates, intro_context)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=3200,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_ai_navigator_brief_result(text, task["candidates"], intro_context)
    return {"title": out["title"], "intro": out["intro"], "summary": out["summary"], "ending": out["ending"], "items": out["items"], "llm": _llm_meta(model, "ai_navigator_brief", usage)}


async def generate_item_navigator_async(persona: str, article: dict, model: str, api_key: str) -> dict:
    task = build_item_navigator_task(persona, article)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2200,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_item_navigator_result(text, task["article"])
    return {"headline": out["headline"], "commentary": out["commentary"], "stance_tags": out["stance_tags"], "llm": _llm_meta(model, "item_navigator", usage)}


async def generate_audio_briefing_script_async(
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
    api_key_hash = hashlib.sha256((api_key or "").encode("utf-8")).hexdigest()[:16]
    cache_key = None
    if _audio_briefing_script_context_cache_enabled():
        cache_key = _cache_key_hash(
            [
                _normalize_model_name(model),
                "audio-briefing-script-v1",
                api_key_hash,
                task["system_instruction"],
            ]
        )
    text, usage = await _generate_content_async(
        task["user_prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=_audio_briefing_script_max_tokens(task["target_chars"], str((intro_context or {}).get("audio_briefing_conversation_mode") or "single")),
        system_instruction=task["system_instruction"],
        context_cache_key=cache_key,
        response_schema=task["schema"],
    )
    out = parse_audio_briefing_script_result(
        text,
        task["articles"],
        persona,
        conversation_mode=str((intro_context or {}).get("audio_briefing_conversation_mode") or "single"),
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
        "turns": out["turns"],
        "ending": out["ending"],
        "llm": _llm_meta(model, "audio_briefing_script", usage),
    }


async def generate_ask_navigator_async(persona: str, ask_input: dict, model: str, api_key: str) -> dict:
    task = build_ask_navigator_task(persona, ask_input)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2400,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_ask_navigator_result(text, task["input"])
    return {"headline": out["headline"], "commentary": out["commentary"], "next_angles": out["next_angles"], "llm": _llm_meta(model, "ask_navigator", usage)}


async def generate_source_navigator_async(persona: str, candidates: list[dict], model: str, api_key: str) -> dict:
    task = build_source_navigator_task(persona, candidates)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2600,
        response_schema=task["schema"],
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    out = parse_source_navigator_result(text, task["candidates"])
    return {"overview": out["overview"], "keep": out["keep"], "watch": out["watch"], "standout": out["standout"], "llm": _llm_meta(model, "source_navigator", usage)}


async def suggest_feed_seed_sites_async(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None,
    negative_examples: list[dict] | None,
    model: str,
    api_key: str,
) -> dict:
    task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=2200,
        response_schema=task["schema"],
    )
    out = parse_seed_sites_result(text, task["existing_sources"])
    if len(out) == 0:
        rescue_prompt = f"""既存ソースと重複しないサイトURL候補を必ず10件以上返してください。JSONのみ。
{{
  "items": [
    {{"url":"https://...", "reason":"..."}}
  ]
}}
既存ソース:
{json.dumps(task["existing_sources"], ensure_ascii=False)}
興味トピック:
{json.dumps(task["preferred_topics"], ensure_ascii=False)}
"""
        rescue_text, rescue_usage = await _generate_content_async(
            rescue_prompt,
            model=model,
            api_key=api_key,
            max_output_tokens=1800,
            response_schema=task["schema"],
        )
        out.extend(parse_seed_sites_result(rescue_text, task["existing_sources"]))
        usage["input_tokens"] = int(usage.get("input_tokens", 0)) + int(rescue_usage.get("input_tokens", 0))
        usage["output_tokens"] = int(usage.get("output_tokens", 0)) + int(rescue_usage.get("output_tokens", 0))
    return {"items": out, "llm": _llm_meta(model, "source_suggestion", usage)}


async def extract_facts_async(title: str | None, content: str, model: str, api_key: str) -> dict:
    task = build_facts_task(title, content, output_mode="array")
    text, usage = await _generate_content_async(task["prompt"], model=model, api_key=api_key, max_output_tokens=1500, system_instruction=task["system_instruction"])
    facts = parse_facts_result(text)
    localization_llm = None
    if not facts:
        raise RuntimeError(f"gemini extract_facts parse failed: response_snippet={text[:500]}")
    if _facts_need_japanese_localization(facts):
        localize_task = build_facts_localization_task(title, facts)
        localized_text, localized_usage = await _generate_content_async(
            localize_task["prompt"],
            model=model,
            api_key=api_key,
            max_output_tokens=1200,
            system_instruction=localize_task["system_instruction"],
            response_schema=localize_task["schema"],
        )
        localized_facts = parse_facts_result(localized_text)
        if localized_facts:
            facts = localized_facts
            localization_llm = _llm_meta(model, "facts_localization", localized_usage)
    return {"facts": facts, "llm": _llm_meta(model, "facts", usage), "facts_localization_llm": localization_llm}


async def summarize_async(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    model: str = "gemini-2.5-flash",
    api_key: str = "",
) -> dict:
    task = build_summary_task(title, facts, source_text_chars)
    max_tokens = _summary_max_tokens(task["target_chars"])
    api_key_hash = hashlib.sha256((api_key or "").encode("utf-8")).hexdigest()[:16]
    cache_key = None
    if _summary_context_cache_enabled():
        cache_key = _cache_key_hash([_normalize_model_name(model), "summary-v2", api_key_hash, task["system_instruction"]])
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=max_tokens,
        system_instruction=task["system_instruction"],
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
    return await asyncio.to_thread(
        finalize_summary_result,
        title=title,
        summary_text=str(data.get("summary", "")).strip(),
        topics=topics,
        raw_score_breakdown=data.get("score_breakdown") if isinstance(data.get("score_breakdown"), dict) else {},
        score_reason=str(data.get("score_reason") or "").strip(),
        translated_title=str(data.get("translated_title") or "").strip(),
        translate_func=lambda raw_title: _translate_title_to_ja(raw_title, model=model, api_key=api_key),
        llm=_llm_meta(model, "summary", usage),
        error_prefix="gemini summarize parse failed",
        response_text=text,
    )


async def check_summary_faithfulness_async(title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
    return await asyncio.to_thread(
        run_summary_faithfulness_check,
        lambda: wrap_usage_transport(
            lambda: _generate_content(
                summary_faithfulness_prompt(title, facts, summary),
                model=model,
                api_key=api_key,
                max_output_tokens=320,
                system_instruction=summary_faithfulness_system_instruction(),
                response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
            ),
            lambda usage: _llm_meta(model, "faithfulness_check", usage),
        ),
        retry_call=lambda: wrap_usage_transport(
            lambda: _generate_content(
                summary_faithfulness_retry_prompt(title, facts, summary),
                model=model,
                api_key=api_key,
                max_output_tokens=120,
                system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                response_schema=None,
                response_mime_type="text/plain",
            ),
            lambda usage: _llm_meta(model, "faithfulness_check", usage),
        ),
    )


async def check_facts_async(title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
    return await asyncio.to_thread(
        run_facts_check,
        lambda: wrap_usage_transport(
            lambda: _generate_content(
                facts_check_prompt(title, content, facts),
                model=model,
                api_key=api_key,
                max_output_tokens=320,
                system_instruction=facts_check_system_instruction(),
                response_schema=FACTS_CHECK_SCHEMA,
            ),
            lambda usage: _llm_meta(model, "facts_check", usage),
        ),
        retry_call=lambda: wrap_usage_transport(
            lambda: _generate_content(
                facts_check_retry_prompt(title, content, facts),
                model=model,
                api_key=api_key,
                max_output_tokens=220,
                system_instruction=facts_check_system_instruction(),
                response_schema=FACTS_CHECK_SCHEMA,
            ),
            lambda usage: _llm_meta(model, "facts_check", usage),
        ),
    )


async def translate_title_async(title: str, model: str = "gemini-2.5-flash", api_key: str = "") -> dict:
    src = (title or "").strip()
    if not src:
        return {"translated_title": "", "llm": None}
    try:
        translated = await asyncio.to_thread(_translate_title_to_ja, src, model, api_key)
    except Exception as e:
        fallback_model = "gemini-2.5-flash-lite"
        if _normalize_model_family(model) == fallback_model:
            raise
        _log.warning("gemini translate_title failed with model=%s, fallback=%s, err=%s", model, fallback_model, e)
        translated = await asyncio.to_thread(_translate_title_to_ja, src, fallback_model, api_key)
    return {
        "translated_title": translated[:300],
        "llm": None,
    }


async def compose_digest_async(digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
    if not items:
        return {
            "subject": f"Sifto Digest - {digest_date}",
            "body": "本日のダイジェスト対象記事はありませんでした。",
            "llm": _llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
        }
    input_mode, digest_input = _build_digest_input_sections(items)
    task = build_digest_task(digest_date, len(items), digest_input, input_mode=input_mode)

    compose_timeout = _env_timeout_seconds("GEMINI_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0)
    last_text = ""
    last_error = "unknown"
    for max_tokens in (10000, 15000):
        text, usage = await _generate_content_async(
            task["prompt"],
            model=model,
            api_key=api_key,
            max_output_tokens=max_tokens,
            response_schema=task["schema"],
            timeout_sec=compose_timeout,
            system_instruction=task["system_instruction"],
        )
        last_text = text
        try:
            subject, body = parse_digest_result(text, error_prefix="gemini compose_digest parse failed")
        except Exception:
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


async def ask_question_async(query: str, candidates: list[dict], model: str, api_key: str) -> dict:
    if not candidates:
        return {
            "answer": "該当する記事は見つかりませんでした。",
            "bullets": [],
            "citations": [],
            "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0}),
        }
    task = build_ask_task(query, candidates)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=ASK_MAX_OUTPUT_TOKENS,
        response_schema=task["schema"],
        timeout_sec=_env_timeout_seconds("GEMINI_TIMEOUT_SEC", 300.0),
        system_instruction=task["system_instruction"],
    )
    result = parse_ask_result(text, candidates, error_prefix="gemini ask missing answer")
    return {**result, "llm": _llm_meta(model, "ask", usage)}


async def ask_rerank_async(query: str, candidates: list[dict], top_k: int, model: str, api_key: str) -> dict:
    if not candidates:
        return {"items": [], "llm": _llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0})}
    task = build_ask_rerank_task(query, candidates, top_k)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=1600,
        response_schema=task["schema"],
        timeout_sec=_env_timeout_seconds("GEMINI_TIMEOUT_SEC", 300.0),
    )
    result = parse_ask_rerank_result(text, candidates, task["top_k"])
    return {**result, "llm": _llm_meta(model, "ask", usage)}


async def compose_digest_cluster_draft_async(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    model: str,
    api_key: str,
) -> dict:
    task = build_cluster_draft_task(cluster_label, item_count, topics, source_lines)
    text, usage = await _generate_content_async(
        task["prompt"],
        model=model,
        api_key=api_key,
        max_output_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
        system_instruction=task["system_instruction"],
        response_schema=task["schema"],
    )
    summary = parse_cluster_draft_result(text, task["source_lines"])
    return {
        "draft_summary": summary,
        "llm": _llm_meta(model, "digest_cluster_draft", usage),
    }
