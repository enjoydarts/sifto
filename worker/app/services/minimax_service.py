import asyncio
import json
import logging
import os
import re
from app.services.llm_catalog import model_pricing
from app.services.anthropic_transport import (
    call_with_model_fallback as _anthropic_call_with_model_fallback,
    call_with_model_fallback_async as _anthropic_call_with_model_fallback_async,
    client_for_api_key as _transport_client_for_api_key,
    async_client_for_api_key as _transport_async_client_for_api_key,
    env_timeout_seconds as _env_timeout_seconds,
    message_text as _message_text,
    message_usage as _message_usage,
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
from app.services.digest_task_common import (
    DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
    build_cluster_draft_task,
    build_digest_task,
    fallback_cluster_draft_from_source_lines,
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
from app.services.task_transport_common import empty_llm_meta, with_execution_failures, wrap_message_fallback_transport, wrap_message_transport

_log = logging.getLogger(__name__)
_MINIMAX_PRICING_SOURCE_VERSION = "llm_catalog"


def _api_base_url() -> str:
    return (os.getenv("MINIMAX_API_BASE_URL") or "https://api.minimax.io/anthropic").strip()


def _client_for_api_key(api_key: str | None):
    return _transport_client_for_api_key(api_key, base_url=_api_base_url())


def _async_client_for_api_key(api_key: str | None):
    return _transport_async_client_for_api_key(api_key, base_url=_api_base_url())


def _call_with_model_fallback(*args, **kwargs):
    return _anthropic_call_with_model_fallback(
        *args,
        base_url=_api_base_url(),
        provider_label="minimax",
        logger=_log,
        **kwargs,
    )


def _with_execution_failures(llm: dict, execution_failures: list[dict] | None) -> dict:
    return with_execution_failures(llm, execution_failures)


def _require_model(model: str | None, purpose: str) -> str:
    resolved = str(model or "").strip()
    if resolved:
        return resolved
    raise RuntimeError(f"minimax model is required for {purpose}")


def _require_api_key(api_key: str | None, purpose: str) -> None:
    if _client_for_api_key(api_key) is None:
        raise RuntimeError(f"minimax api key is required for {purpose}")


def _format_execution_failures(execution_failures: list[dict] | None) -> str:
    failures = execution_failures or []
    parts: list[str] = []
    for failure in failures:
        if not isinstance(failure, dict):
            continue
        reason = str(failure.get("reason") or "").strip()
        model = str(failure.get("model") or "").strip()
        if reason and model:
            parts.append(f"{model}: {reason}")
        elif reason:
            parts.append(reason)
    return " | ".join(parts)


def _raise_execution_failure(purpose: str, execution_failures: list[dict] | None, default_message: str) -> None:
    detail = _format_execution_failures(execution_failures)
    if detail:
        raise RuntimeError(f"minimax {purpose} failed: {detail}")
    raise RuntimeError(default_message)


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
                str(((_extract_first_json_object(_message_text(message)) or {}).get("translated_title") or ""))
                if message is not None
                else ""
            )
        )(
            _call_with_model_fallback(
                prompt,
                model,
                None,
                max_tokens=200,
                api_key=api_key,
            )[0]
        ),
        plain_retry_call=lambda: (
            lambda message: (_message_text(message) if message is not None else "")
        )(
            _call_with_model_fallback(
                plain_prompt,
                model,
                None,
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
            "pricing_source": _MINIMAX_PRICING_SOURCE_VERSION,
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

    execution_failures = []
    for meta in valid:
        failures = meta.get("execution_failures")
        if isinstance(failures, list):
            execution_failures.extend(f for f in failures if isinstance(f, dict))

    merged = {
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
    if execution_failures:
        merged["execution_failures"] = execution_failures
    return merged


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
    normalized = model.removeprefix("minimax::").removeprefix("minimax/")
    if model_pricing(normalized) is not None:
        return normalized
    return model


def _pricing_for_model(model: str, purpose: str) -> dict:
    family = _normalize_model_family(model)
    base = dict(
        model_pricing(family)
        or model_pricing(model)
        or {
            "input_per_mtok_usd": 0.0,
            "output_per_mtok_usd": 0.0,
            "cache_write_per_mtok_usd": 0.0,
            "cache_read_per_mtok_usd": 0.0,
        }
    )
    source = str(base.get("pricing_source") or _MINIMAX_PRICING_SOURCE_VERSION)
    # Optional per-purpose overrides for temporary pricing changes without deploy.
    prefix = f"MINIMAX_{purpose.upper()}_"
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


def _estimate_cost_usd(model: str, purpose: str, usage: dict) -> float:
    p = _pricing_for_model(model, purpose)
    total = 0.0
    total += usage["input_tokens"] / 1_000_000 * p["input_per_mtok_usd"]
    total += usage["output_tokens"] / 1_000_000 * p["output_per_mtok_usd"]
    total += usage["cache_creation_input_tokens"] / 1_000_000 * p["cache_write_per_mtok_usd"]
    total += usage["cache_read_input_tokens"] / 1_000_000 * p["cache_read_per_mtok_usd"]
    return round(total, 8)


def _llm_meta(message, purpose: str, model: str, provider: str = "minimax") -> dict:
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

def extract_facts(title: str | None, content: str, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "facts")
    _require_api_key(api_key, "facts")

    chunks = _split_text_chunks(content, chunk_chars=8000, overlap_chars=400)
    if not chunks:
        return {"facts": [], "llm": _merge_llm_metas([], "facts")}

    all_fact_lists: list[list[str]] = []
    llm_metas: list[dict] = []
    execution_failures_all: list[dict] = []
    any_llm_success = False
    per_chunk_fact_target = 4 if len(chunks) <= 3 else 3 if len(chunks) <= 8 else 2

    for idx, chunk in enumerate(chunks, start=1):
        task = build_facts_task(
            title,
            f"チャンク: {idx}/{len(chunks)}\n\n{chunk}",
            output_mode="array",
            fact_range=f"{per_chunk_fact_target}〜{per_chunk_fact_target + 2}個",
        )
        message, used_model, execution_failures = _call_with_model_fallback(
            f"{task['system_instruction']}\n\n{task['prompt']}",
            resolved_model,
            None,
            max_tokens=1024,
            api_key=api_key,
            system_prompt=task["system_instruction"],
            user_prompt=task["prompt"],
        )
        execution_failures_all.extend(execution_failures or [])
        if message is None:
            continue
        any_llm_success = True
        text = _message_text(message)
        all_fact_lists.append(parse_facts_result(text))
        llm_metas.append(_with_execution_failures(_llm_meta(message, "facts", used_model or resolved_model), execution_failures))

    if not any_llm_success:
        _raise_execution_failure("facts", execution_failures_all, "minimax facts returned no message")

    merged_facts = _merge_fact_lists(all_fact_lists, max_items=24)
    localization_llm = None
    if merged_facts and _facts_need_japanese_localization(merged_facts):
        localize_task = build_facts_localization_task(title, merged_facts)
        message, used_model, execution_failures = _call_with_model_fallback(
            f"{localize_task['system_instruction']}\n\n{localize_task['prompt']}",
            resolved_model,
            None,
            max_tokens=1024,
            api_key=api_key,
            system_prompt=localize_task["system_instruction"],
            user_prompt=localize_task["prompt"],
        )
        execution_failures_all.extend(execution_failures or [])
        if message is not None:
            localized_facts = parse_facts_result(_message_text(message))
            if localized_facts:
                merged_facts = localized_facts
                localization_llm = _with_execution_failures(_llm_meta(message, "facts_localization", used_model or resolved_model), execution_failures)
    llm = _merge_llm_metas(llm_metas, "facts")
    llm["chunk_count"] = len(chunks)
    llm["chunk_success_count"] = len(llm_metas)
    return {
        "facts": merged_facts,
        "llm": llm,
        "facts_localization_llm": localization_llm,
    }


def summarize(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "summary")
    task = build_summary_task(title, facts, source_text_chars)
    max_tokens = _summary_max_tokens(task["target_chars"])
    _require_api_key(api_key, "summary")

    prompt = f"{task['system_instruction']}\n\n{task['prompt']}"
    enable_summary_prompt_cache = os.getenv("MINIMAX_SUMMARY_PROMPT_CACHE", "0").strip() not in ("0", "false", "False")
    message, used_model, execution_failures = _call_with_model_fallback(
        prompt,
        resolved_model,
        None,
        max_tokens=max_tokens,
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
        enable_prompt_cache=enable_summary_prompt_cache,
    )
    if message is None:
        _raise_execution_failure("summary", execution_failures, "minimax summary returned no message")
    text = _message_text(message)
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
        genre=str(data.get("genre") or "").strip(),
        other_label=str(data.get("other_label") or "").strip(),
        raw_score_breakdown=data.get("score_breakdown") if isinstance(data.get("score_breakdown"), dict) else {},
        score_reason=str(data.get("score_reason") or "").strip(),
        translated_title=str(data.get("translated_title") or "").strip(),
        translate_func=lambda raw_title: _translate_title_to_ja(raw_title, used_model or resolved_model, api_key=api_key),
        llm=_with_execution_failures(_llm_meta(message, "summary", used_model or resolved_model), execution_failures),
        error_prefix="minimax summarize parse failed",
        response_text=text,
    )


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "faithfulness_check")
    _require_api_key(api_key, "faithfulness_check")
    prompt = summary_faithfulness_prompt(title, facts, summary)
    message, used_model, _execution_failures = _call_with_model_fallback(
        prompt,
        resolved_model,
        None,
        max_tokens=320,
        api_key=api_key,
        system_prompt=summary_faithfulness_system_instruction(),
        user_prompt=prompt,
    )
    if message is None:
        _raise_execution_failure("faithfulness_check", _execution_failures, "minimax faithfulness_check returned no message")
    return run_summary_faithfulness_check(
        lambda: wrap_message_transport(
            message,
            lambda msg: _llm_meta(msg, "faithfulness_check", used_model or resolved_model),
            empty_llm_meta("minimax", used_model or resolved_model, _MINIMAX_PRICING_SOURCE_VERSION),
        ),
        retry_call=lambda: wrap_message_fallback_transport(
            _call_with_model_fallback(
                summary_faithfulness_retry_prompt(title, facts, summary),
                resolved_model,
                None,
                max_tokens=120,
                api_key=api_key,
                system_prompt="pass / warn / fail のいずれか1語のみを返す。",
                user_prompt=summary_faithfulness_retry_prompt(title, facts, summary),
                enable_prompt_cache=False,
            ),
            lambda msg, resolved_model: _llm_meta(msg, "faithfulness_check", resolved_model),
            "minimax",
            used_model or resolved_model,
            _MINIMAX_PRICING_SOURCE_VERSION,
        ),
    )


def check_facts(title: str | None, content: str, facts: list[str], api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "facts_check")
    _require_api_key(api_key, "facts_check")
    prompt = facts_check_prompt(title, content, facts)
    message, used_model, _execution_failures = _call_with_model_fallback(
        prompt,
        resolved_model,
        None,
        max_tokens=320,
        api_key=api_key,
        system_prompt=facts_check_system_instruction(),
        user_prompt=prompt,
    )
    if message is None:
        _raise_execution_failure("facts_check", _execution_failures, "minimax facts_check returned no message")
    retry_prompt = facts_check_retry_prompt(title, content, facts)
    return run_facts_check(
        lambda: wrap_message_transport(
            message,
            lambda msg: _llm_meta(msg, "facts_check", used_model or resolved_model),
            empty_llm_meta("minimax", used_model or resolved_model, _MINIMAX_PRICING_SOURCE_VERSION),
        ),
        retry_call=lambda: wrap_message_fallback_transport(
            _call_with_model_fallback(
                retry_prompt,
                resolved_model,
                None,
                max_tokens=220,
                api_key=api_key,
                system_prompt=facts_check_system_instruction(),
                user_prompt=retry_prompt,
                enable_prompt_cache=False,
            ),
            lambda msg, resolved_model: _llm_meta(msg, "facts_check", resolved_model),
            "minimax",
            used_model or resolved_model,
            _MINIMAX_PRICING_SOURCE_VERSION,
        ),
    )


def translate_title(title: str, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "summary")
    _require_api_key(api_key, "summary")
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
    message, used_model, _execution_failures = _call_with_model_fallback(
        prompt,
        resolved_model,
        None,
        max_tokens=200,
        api_key=api_key,
    )
    if message is None:
        _raise_execution_failure("summary", _execution_failures, "minimax translate_title returned no message")

    text = _message_text(message)
    data = _extract_first_json_object(text) or {}
    translated = str(data.get("translated_title") or "").strip()
    if not translated:
        translated = _translate_title_to_ja(src, used_model or resolved_model, api_key=api_key)
    return {
        "translated_title": translated[:300],
        "llm": _llm_meta(message, "summary", used_model or resolved_model),
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
    resolved_model = _require_model(model, "digest")
    _require_api_key(api_key, "digest")
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
    compose_timeout = _env_timeout_seconds("MINIMAX_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0)

    task = build_digest_task(digest_date, len(items), digest_input, input_mode=input_mode)

    message, used_model, _execution_failures = _call_with_model_fallback(
        f"{task['system_instruction']}\n\n{task['prompt']}",
        resolved_model,
        None,
        max_tokens=10000,
        api_key=api_key,
        timeout_sec=compose_timeout,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
    )
    if message is None:
        _raise_execution_failure("digest", _execution_failures, "minimax digest returned no message")

    text = _message_text(message)
    subject, body = parse_digest_result(text, error_prefix="claude compose_digest missing subject/body")
    if len(body) < 80:
        raise RuntimeError(f"claude compose_digest body too short: len={len(body)}")
    llm = _llm_meta(message, "digest", used_model or resolved_model)
    llm["input_mode"] = input_mode
    llm["items_count"] = len(items)
    return {
        "subject": subject,
        "body": body,
        "llm": llm,
    }


def ask_question(query: str, candidates: list[dict], api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "ask")
    _require_api_key(api_key, "ask")
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
    task = build_ask_task(query, candidates)
    message, used_model, _execution_failures = _call_with_model_fallback(
        f"{task['system_instruction']}\n\n{task['prompt']}",
        resolved_model,
        None,
        max_tokens=ASK_MAX_OUTPUT_TOKENS,
        api_key=api_key,
        timeout_sec=_env_timeout_seconds("MINIMAX_TIMEOUT_SEC", 300.0),
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
    )
    if message is None:
        _raise_execution_failure("ask", _execution_failures, "minimax ask returned no message")
    text = _message_text(message)
    result = parse_ask_result(text, candidates, error_prefix="claude ask missing answer")
    return {**result, "llm": _llm_meta(message, "ask", used_model or resolved_model)}


def ask_rerank(query: str, candidates: list[dict], top_k: int, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "ask")
    _require_api_key(api_key, "ask")
    if not candidates:
        return {"items": [], "llm": empty_llm_meta("none", "none")}
    task = build_ask_rerank_task(query, candidates, top_k)
    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=1600,
        api_key=api_key,
        timeout_sec=_env_timeout_seconds("MINIMAX_TIMEOUT_SEC", 300.0),
    )
    if message is None:
        _raise_execution_failure("ask", _execution_failures, "minimax ask_rerank returned no message")
    text = _message_text(message)
    result = parse_ask_rerank_result(text, candidates, task["top_k"])
    return {**result, "llm": _llm_meta(message, "ask", used_model or resolved_model)}


def compose_digest_cluster_draft(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "digest_cluster_draft")
    _require_api_key(api_key, "digest_cluster_draft")
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

    task = build_cluster_draft_task(cluster_label, item_count, topics, source_lines)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
    )
    if message is None:
        _raise_execution_failure("digest_cluster_draft", _execution_failures, "minimax digest_cluster_draft returned no message")
    text = _message_text(message)
    draft_summary = parse_cluster_draft_result(text, task["source_lines"])
    return {
        "draft_summary": draft_summary,
        "llm": _llm_meta(message, "digest_cluster_draft", used_model or resolved_model),
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
    resolved_model = _require_model(model, "source_suggestion")
    _require_api_key(api_key, "source_suggestion")
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
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2800,
        api_key=api_key,
    )
    if message is None:
        _raise_execution_failure("source_suggestion", _execution_failures, "minimax source_suggestion returned no message")

    text = _message_text(message)
    out = parse_rank_feed_result(text, task["candidates"])
    return {
        "items": out,
        "llm": _llm_meta(message, "source_suggestion", used_model or resolved_model),
    }


def generate_briefing_navigator(
    persona: str,
    candidates: list[dict],
    intro_context: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "briefing_navigator")
    _require_api_key(api_key, "briefing_navigator")
    task = build_briefing_navigator_task(persona, candidates, intro_context)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=1800,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("briefing_navigator", _execution_failures, "minimax briefing_navigator returned no message")

    text = _message_text(message)
    out = parse_briefing_navigator_result(text, task["candidates"])
    return {
        "intro": out["intro"],
        "picks": out["picks"],
        "llm": _llm_meta(message, "briefing_navigator", used_model or resolved_model),
    }


def compose_ai_navigator_brief(
    persona: str,
    candidates: list[dict],
    intro_context: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "ai_navigator_brief")
    _require_api_key(api_key, "ai_navigator_brief")
    task = build_ai_navigator_brief_task(persona, candidates, intro_context)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=3200,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("ai_navigator_brief", _execution_failures, "minimax ai_navigator_brief returned no message")

    text = _message_text(message)
    out = parse_ai_navigator_brief_result(text, task["candidates"], intro_context)
    return {
        "title": out["title"],
        "intro": out["intro"],
        "summary": out["summary"],
        "ending": out["ending"],
        "items": out["items"],
        "llm": _llm_meta(message, "ai_navigator_brief", used_model or resolved_model),
    }


def generate_item_navigator(
    persona: str,
    article: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "item_navigator")
    _require_api_key(api_key, "item_navigator")
    task = build_item_navigator_task(persona, article)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2200,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("item_navigator", _execution_failures, "minimax item_navigator returned no message")

    text = _message_text(message)
    out = parse_item_navigator_result(text, task["article"])
    return {
        "headline": out["headline"],
        "commentary": out["commentary"],
        "stance_tags": out["stance_tags"],
        "llm": _llm_meta(message, "item_navigator", used_model or resolved_model),
    }


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
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "audio_briefing_script")
    _require_api_key(api_key, "audio_briefing_script")
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
    message, used_model, _execution_failures = _call_with_model_fallback(
        task["user_prompt"],
        resolved_model,
        None,
        max_tokens=_audio_briefing_script_max_tokens(task["target_chars"], str((intro_context or {}).get("audio_briefing_conversation_mode") or "single")),
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["user_prompt"],
        enable_prompt_cache=os.getenv("MINIMAX_AUDIO_BRIEFING_SCRIPT_PROMPT_CACHE", "1").strip() not in ("0", "false", "False"),
    )
    if message is None:
        raise RuntimeError("audio briefing script returned no message")

    text = _message_text(message)
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
        "llm": _llm_meta(message, "audio_briefing_script", used_model or resolved_model),
    }


def generate_ask_navigator(
    persona: str,
    ask_input: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "ask_navigator")
    _require_api_key(api_key, "ask_navigator")
    task = build_ask_navigator_task(persona, ask_input)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2400,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("ask_navigator", _execution_failures, "minimax ask_navigator returned no message")

    text = _message_text(message)
    out = parse_ask_navigator_result(text, task["input"])
    return {
        "headline": out["headline"],
        "commentary": out["commentary"],
        "next_angles": out["next_angles"],
        "llm": _llm_meta(message, "ask_navigator", used_model or resolved_model),
    }


def generate_source_navigator(
    persona: str,
    candidates: list[dict],
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "source_navigator")
    _require_api_key(api_key, "source_navigator")
    task = build_source_navigator_task(persona, candidates)

    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2600,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("source_navigator", _execution_failures, "minimax source_navigator returned no message")

    text = _message_text(message)
    out = parse_source_navigator_result(text, task["candidates"])
    return {
        "overview": out["overview"],
        "keep": out["keep"],
        "watch": out["watch"],
        "standout": out["standout"],
        "llm": _llm_meta(message, "source_navigator", used_model or resolved_model),
    }


def suggest_feed_seed_sites(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None = None,
    negative_examples: list[dict] | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "source_suggestion")
    _require_api_key(api_key, "source_suggestion")
    task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
    message, used_model, _execution_failures = _call_with_model_fallback(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2200,
        api_key=api_key,
    )
    if message is None:
        _raise_execution_failure("source_suggestion", _execution_failures, "minimax source_suggestion returned no message")
    text = _message_text(message)
    out = parse_seed_sites_result(text, task["existing_sources"])
    return {
        "items": out,
        "llm": _llm_meta(message, "source_suggestion", used_model or resolved_model),
    }


async def _call_with_model_fallback_async(*args, **kwargs):
    return await _anthropic_call_with_model_fallback_async(
        *args,
        base_url=_api_base_url(),
        provider_label="minimax",
        logger=_log,
        **kwargs,
    )


async def extract_facts_async(title: str | None, content: str, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "facts")
    _require_api_key(api_key, "facts")

    chunks = _split_text_chunks(content, chunk_chars=8000, overlap_chars=400)
    if not chunks:
        return {"facts": [], "llm": _merge_llm_metas([], "facts")}

    all_fact_lists: list[list[str]] = []
    llm_metas: list[dict] = []
    execution_failures_all: list[dict] = []
    any_llm_success = False
    per_chunk_fact_target = 4 if len(chunks) <= 3 else 3 if len(chunks) <= 8 else 2

    for idx, chunk in enumerate(chunks, start=1):
        task = build_facts_task(
            title,
            f"チャンク: {idx}/{len(chunks)}\n\n{chunk}",
            output_mode="array",
            fact_range=f"{per_chunk_fact_target}〜{per_chunk_fact_target + 2}個",
        )
        message, used_model, execution_failures = await _call_with_model_fallback_async(
            f"{task['system_instruction']}\n\n{task['prompt']}",
            resolved_model,
            None,
            max_tokens=1024,
            api_key=api_key,
            system_prompt=task["system_instruction"],
            user_prompt=task["prompt"],
        )
        execution_failures_all.extend(execution_failures or [])
        if message is None:
            continue
        any_llm_success = True
        text = _message_text(message)
        all_fact_lists.append(parse_facts_result(text))
        llm_metas.append(_with_execution_failures(_llm_meta(message, "facts", used_model or resolved_model), execution_failures))

    if not any_llm_success:
        _raise_execution_failure("facts", execution_failures_all, "minimax facts returned no message")

    merged_facts = _merge_fact_lists(all_fact_lists, max_items=24)
    localization_llm = None
    if merged_facts and _facts_need_japanese_localization(merged_facts):
        localize_task = build_facts_localization_task(title, merged_facts)
        message, used_model, execution_failures = await _call_with_model_fallback_async(
            f"{localize_task['system_instruction']}\n\n{localize_task['prompt']}",
            resolved_model,
            None,
            max_tokens=1024,
            api_key=api_key,
            system_prompt=localize_task["system_instruction"],
            user_prompt=localize_task["prompt"],
        )
        execution_failures_all.extend(execution_failures or [])
        if message is not None:
            localized_facts = parse_facts_result(_message_text(message))
            if localized_facts:
                merged_facts = localized_facts
                localization_llm = _with_execution_failures(_llm_meta(message, "facts_localization", used_model or resolved_model), execution_failures)
    llm = _merge_llm_metas(llm_metas, "facts")
    llm["chunk_count"] = len(chunks)
    llm["chunk_success_count"] = len(llm_metas)
    return {
        "facts": merged_facts,
        "llm": llm,
        "facts_localization_llm": localization_llm,
    }


async def summarize_async(
    title: str | None,
    facts: list[str],
    source_text_chars: int | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "summary")
    task = build_summary_task(title, facts, source_text_chars)
    max_tokens = _summary_max_tokens(task["target_chars"])
    _require_api_key(api_key, "summary")

    prompt = f"{task['system_instruction']}\n\n{task['prompt']}"
    enable_summary_prompt_cache = os.getenv("MINIMAX_SUMMARY_PROMPT_CACHE", "0").strip() not in ("0", "false", "False")
    message, used_model, execution_failures = await _call_with_model_fallback_async(
        prompt,
        resolved_model,
        None,
        max_tokens=max_tokens,
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
        enable_prompt_cache=enable_summary_prompt_cache,
    )
    if message is None:
        _raise_execution_failure("summary", execution_failures, "minimax summary returned no message")
    text = _message_text(message)
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
        summary_text=str(data.get("summary", "")),
        topics=topics,
        raw_score_breakdown=data.get("score_breakdown") if isinstance(data.get("score_breakdown"), dict) else {},
        score_reason=str(data.get("score_reason") or "").strip(),
        translated_title=str(data.get("translated_title") or "").strip(),
        translate_func=lambda raw_title: _translate_title_to_ja(raw_title, used_model or resolved_model, api_key=api_key),
        llm=_with_execution_failures(_llm_meta(message, "summary", used_model or resolved_model), execution_failures),
        error_prefix="minimax summarize parse failed",
        response_text=text,
    )


async def check_summary_faithfulness_async(title: str | None, facts: list[str], summary: str, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "faithfulness_check")
    _require_api_key(api_key, "faithfulness_check")
    prompt = summary_faithfulness_prompt(title, facts, summary)
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        prompt,
        resolved_model,
        None,
        max_tokens=320,
        api_key=api_key,
        system_prompt=summary_faithfulness_system_instruction(),
        user_prompt=prompt,
    )
    if message is None:
        _raise_execution_failure("faithfulness_check", _execution_failures, "minimax faithfulness_check returned no message")
    return await asyncio.to_thread(
        run_summary_faithfulness_check,
        lambda: wrap_message_transport(
            message,
            lambda msg: _llm_meta(msg, "faithfulness_check", used_model or resolved_model),
            empty_llm_meta("minimax", used_model or resolved_model, _MINIMAX_PRICING_SOURCE_VERSION),
        ),
        retry_call=lambda: wrap_message_fallback_transport(
            _call_with_model_fallback(
                summary_faithfulness_retry_prompt(title, facts, summary),
                resolved_model,
                None,
                max_tokens=120,
                api_key=api_key,
                system_prompt="pass / warn / fail のいずれか1語のみを返す。",
                user_prompt=summary_faithfulness_retry_prompt(title, facts, summary),
                enable_prompt_cache=False,
            ),
            lambda msg, resolved_model: _llm_meta(msg, "faithfulness_check", resolved_model),
            "minimax",
            used_model or resolved_model,
            _MINIMAX_PRICING_SOURCE_VERSION,
        ),
    )


async def check_facts_async(title: str | None, content: str, facts: list[str], api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "facts_check")
    _require_api_key(api_key, "facts_check")
    prompt = facts_check_prompt(title, content, facts)
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        prompt,
        resolved_model,
        None,
        max_tokens=320,
        api_key=api_key,
        system_prompt=facts_check_system_instruction(),
        user_prompt=prompt,
    )
    if message is None:
        _raise_execution_failure("facts_check", _execution_failures, "minimax facts_check returned no message")
    retry_prompt = facts_check_retry_prompt(title, content, facts)
    return await asyncio.to_thread(
        run_facts_check,
        lambda: wrap_message_transport(
            message,
            lambda msg: _llm_meta(msg, "facts_check", used_model or resolved_model),
            empty_llm_meta("minimax", used_model or resolved_model, _MINIMAX_PRICING_SOURCE_VERSION),
        ),
        retry_call=lambda: wrap_message_fallback_transport(
            _call_with_model_fallback(
                retry_prompt,
                resolved_model,
                None,
                max_tokens=220,
                api_key=api_key,
                system_prompt=facts_check_system_instruction(),
                user_prompt=retry_prompt,
                enable_prompt_cache=False,
            ),
            lambda msg, resolved_model: _llm_meta(msg, "facts_check", resolved_model),
            "minimax",
            used_model or resolved_model,
            _MINIMAX_PRICING_SOURCE_VERSION,
        ),
    )


async def translate_title_async(title: str, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "summary")
    _require_api_key(api_key, "summary")
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
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        prompt,
        resolved_model,
        None,
        max_tokens=200,
        api_key=api_key,
    )
    if message is None:
        _raise_execution_failure("summary", _execution_failures, "minimax translate_title returned no message")

    text = _message_text(message)
    data = _extract_first_json_object(text) or {}
    translated = str(data.get("translated_title") or "").strip()
    if not translated:
        translated = await asyncio.to_thread(_translate_title_to_ja, src, used_model or resolved_model, api_key)
    return {
        "translated_title": translated[:300],
        "llm": _llm_meta(message, "summary", used_model or resolved_model),
    }


async def compose_digest_async(digest_date: str, items: list[dict], api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "digest")
    _require_api_key(api_key, "digest")
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
    compose_timeout = _env_timeout_seconds("MINIMAX_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0)

    task = build_digest_task(digest_date, len(items), digest_input, input_mode=input_mode)

    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        f"{task['system_instruction']}\n\n{task['prompt']}",
        resolved_model,
        None,
        max_tokens=10000,
        api_key=api_key,
        timeout_sec=compose_timeout,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
    )
    if message is None:
        _raise_execution_failure("digest", _execution_failures, "minimax digest returned no message")

    text = _message_text(message)
    subject, body = parse_digest_result(text, error_prefix="claude compose_digest missing subject/body")
    if len(body) < 80:
        raise RuntimeError(f"claude compose_digest body too short: len={len(body)}")
    llm = _llm_meta(message, "digest", used_model or resolved_model)
    llm["input_mode"] = input_mode
    llm["items_count"] = len(items)
    return {
        "subject": subject,
        "body": body,
        "llm": llm,
    }


async def ask_question_async(query: str, candidates: list[dict], api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "ask")
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
    task = build_ask_task(query, candidates)
    _require_api_key(api_key, "ask")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        f"{task['system_instruction']}\n\n{task['prompt']}",
        resolved_model,
        None,
        max_tokens=ASK_MAX_OUTPUT_TOKENS,
        api_key=api_key,
        timeout_sec=_env_timeout_seconds("MINIMAX_TIMEOUT_SEC", 300.0),
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
    )
    if message is None:
        _raise_execution_failure("ask", _execution_failures, "minimax ask returned no message")
    text = _message_text(message)
    result = parse_ask_result(text, candidates, error_prefix="claude ask missing answer")
    return {**result, "llm": _llm_meta(message, "ask", used_model or resolved_model)}


async def ask_rerank_async(query: str, candidates: list[dict], top_k: int, api_key: str | None = None, model: str | None = None) -> dict:
    resolved_model = _require_model(model, "ask")
    if not candidates:
        return {"items": [], "llm": empty_llm_meta("none", "none")}
    task = build_ask_rerank_task(query, candidates, top_k)
    _require_api_key(api_key, "ask")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=1600,
        api_key=api_key,
        timeout_sec=_env_timeout_seconds("MINIMAX_TIMEOUT_SEC", 300.0),
    )
    if message is None:
        _raise_execution_failure("ask", _execution_failures, "minimax ask_rerank returned no message")
    text = _message_text(message)
    result = parse_ask_rerank_result(text, candidates, task["top_k"])
    return {**result, "llm": _llm_meta(message, "ask", used_model or resolved_model)}


async def compose_digest_cluster_draft_async(
    cluster_label: str,
    item_count: int,
    topics: list[str],
    source_lines: list[str],
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "digest_cluster_draft")
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
    _require_api_key(api_key, "digest_cluster_draft")
    task = build_cluster_draft_task(cluster_label, item_count, topics, source_lines)

    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["prompt"],
    )
    if message is None:
        _raise_execution_failure("digest_cluster_draft", _execution_failures, "minimax digest_cluster_draft returned no message")
    text = _message_text(message)
    draft_summary = parse_cluster_draft_result(text, task["source_lines"])
    return {
        "draft_summary": draft_summary,
        "llm": _llm_meta(message, "digest_cluster_draft", used_model or resolved_model),
    }


async def rank_feed_suggestions_async(
    existing_sources: list[dict],
    preferred_topics: list[str],
    candidates: list[dict],
    positive_examples: list[dict] | None = None,
    negative_examples: list[dict] | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "source_suggestion")
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
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
    _require_api_key(api_key, "source_suggestion")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2800,
        api_key=api_key,
    )
    if message is None:
        _raise_execution_failure("source_suggestion", _execution_failures, "minimax source_suggestion returned no message")

    text = _message_text(message)
    out = parse_rank_feed_result(text, task["candidates"])
    return {
        "items": out,
        "llm": _llm_meta(message, "source_suggestion", used_model or resolved_model),
    }


async def generate_briefing_navigator_async(
    persona: str,
    candidates: list[dict],
    intro_context: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "briefing_navigator")
    task = build_briefing_navigator_task(persona, candidates, intro_context)
    _require_api_key(api_key, "briefing_navigator")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=1800,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("briefing_navigator", _execution_failures, "minimax briefing_navigator returned no message")

    text = _message_text(message)
    out = parse_briefing_navigator_result(text, task["candidates"])
    return {
        "intro": out["intro"],
        "picks": out["picks"],
        "llm": _llm_meta(message, "briefing_navigator", used_model or resolved_model),
    }


async def compose_ai_navigator_brief_async(
    persona: str,
    candidates: list[dict],
    intro_context: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "ai_navigator_brief")
    task = build_ai_navigator_brief_task(persona, candidates, intro_context)
    _require_api_key(api_key, "ai_navigator_brief")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=3200,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("ai_navigator_brief", _execution_failures, "minimax ai_navigator_brief returned no message")

    text = _message_text(message)
    out = parse_ai_navigator_brief_result(text, task["candidates"], intro_context)
    return {
        "title": out["title"],
        "intro": out["intro"],
        "summary": out["summary"],
        "ending": out["ending"],
        "items": out["items"],
        "llm": _llm_meta(message, "ai_navigator_brief", used_model or resolved_model),
    }


async def generate_item_navigator_async(
    persona: str,
    article: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "item_navigator")
    task = build_item_navigator_task(persona, article)
    _require_api_key(api_key, "item_navigator")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2200,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("item_navigator", _execution_failures, "minimax item_navigator returned no message")

    text = _message_text(message)
    out = parse_item_navigator_result(text, task["article"])
    return {
        "headline": out["headline"],
        "commentary": out["commentary"],
        "stance_tags": out["stance_tags"],
        "llm": _llm_meta(message, "item_navigator", used_model or resolved_model),
    }


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
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "audio_briefing_script")
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
    if _async_client_for_api_key(api_key) is None:
        raise RuntimeError("audio briefing script api client is unavailable")

    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["user_prompt"],
        resolved_model,
        None,
        max_tokens=_audio_briefing_script_max_tokens(task["target_chars"], str((intro_context or {}).get("audio_briefing_conversation_mode") or "single")),
        api_key=api_key,
        system_prompt=task["system_instruction"],
        user_prompt=task["user_prompt"],
        enable_prompt_cache=os.getenv("MINIMAX_AUDIO_BRIEFING_SCRIPT_PROMPT_CACHE", "1").strip() not in ("0", "false", "False"),
    )
    if message is None:
        raise RuntimeError("audio briefing script returned no message")

    text = _message_text(message)
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
        "llm": _llm_meta(message, "audio_briefing_script", used_model or resolved_model),
    }


async def generate_ask_navigator_async(
    persona: str,
    ask_input: dict,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "ask_navigator")
    task = build_ask_navigator_task(persona, ask_input)
    _require_api_key(api_key, "ask_navigator")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2400,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("ask_navigator", _execution_failures, "minimax ask_navigator returned no message")

    text = _message_text(message)
    out = parse_ask_navigator_result(text, task["input"])
    return {
        "headline": out["headline"],
        "commentary": out["commentary"],
        "next_angles": out["next_angles"],
        "llm": _llm_meta(message, "ask_navigator", used_model or resolved_model),
    }


async def generate_source_navigator_async(
    persona: str,
    candidates: list[dict],
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "source_navigator")
    task = build_source_navigator_task(persona, candidates)
    _require_api_key(api_key, "source_navigator")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2600,
        api_key=api_key,
        temperature=task["sampling_profile"]["temperature"],
        top_p=task["sampling_profile"]["top_p"],
    )
    if message is None:
        _raise_execution_failure("source_navigator", _execution_failures, "minimax source_navigator returned no message")

    text = _message_text(message)
    out = parse_source_navigator_result(text, task["candidates"])
    return {
        "overview": out["overview"],
        "keep": out["keep"],
        "watch": out["watch"],
        "standout": out["standout"],
        "llm": _llm_meta(message, "source_navigator", used_model or resolved_model),
    }


async def suggest_feed_seed_sites_async(
    existing_sources: list[dict],
    preferred_topics: list[str],
    positive_examples: list[dict] | None = None,
    negative_examples: list[dict] | None = None,
    api_key: str | None = None,
    model: str | None = None,
) -> dict:
    resolved_model = _require_model(model, "source_suggestion")
    task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
    _require_api_key(api_key, "source_suggestion")
    message, used_model, _execution_failures = await _call_with_model_fallback_async(
        task["prompt"],
        resolved_model,
        None,
        max_tokens=2200,
        api_key=api_key,
    )
    if message is None:
        _raise_execution_failure("source_suggestion", _execution_failures, "minimax source_suggestion returned no message")
    text = _message_text(message)
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
        rescue_message, _, _execution_failures = await _call_with_model_fallback_async(
            rescue_prompt,
            resolved_model,
            None,
            max_tokens=1500,
            api_key=api_key,
        )
        if rescue_message is not None:
            out.extend(parse_seed_sites_result(_message_text(rescue_message), task["existing_sources"]))
    return {
        "items": out,
        "llm": _llm_meta(message, "source_suggestion", used_model or resolved_model),
    }
