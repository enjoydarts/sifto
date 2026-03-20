import logging
import os

from app.services.digest_task_common import (
    build_cluster_draft_task,
    build_digest_task,
    fallback_cluster_draft_from_source_lines,
    build_simple_digest_input,
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
    build_ask_task,
    build_rank_feed_task,
    build_seed_sites_task,
    parse_ask_result,
    parse_rank_feed_result,
    parse_seed_sites_result,
)
from app.services.llm_catalog import model_pricing, model_supports
from app.services.llm_text_utils import (
    extract_first_json_object as _extract_first_json_object,
)
from app.services.openai_compat_transport import run_chat_json
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
_FIREWORKS_PRICING_SOURCE_VERSION = "fireworks_static_2026_03"
_LEGACY_MODEL_PRICING = {
    "fireworks/deepseek-v3p1": {"input_per_mtok_usd": 0.56, "output_per_mtok_usd": 1.68, "cache_read_per_mtok_usd": 0.28},
    "fireworks/deepseek-v3p2": {"input_per_mtok_usd": 0.56, "output_per_mtok_usd": 1.68, "cache_read_per_mtok_usd": 0.28},
    "cogito/cogito-671b-v2-p1": {"input_per_mtok_usd": 1.20, "output_per_mtok_usd": 1.20, "cache_read_per_mtok_usd": 0.60},
    "fireworks/gpt-oss-20b": {"input_per_mtok_usd": 0.07, "output_per_mtok_usd": 0.30, "cache_read_per_mtok_usd": 0.04},
    "fireworks/gpt-oss-120b": {"input_per_mtok_usd": 0.15, "output_per_mtok_usd": 0.60, "cache_read_per_mtok_usd": 0.01},
    "fireworks/glm-4p7": {"input_per_mtok_usd": 0.60, "output_per_mtok_usd": 2.20, "cache_read_per_mtok_usd": 0.30},
    "fireworks/glm-5": {"input_per_mtok_usd": 1.0, "output_per_mtok_usd": 3.2, "cache_read_per_mtok_usd": 0.2},
    "fireworks/kimi-k2-instruct-0905": {"input_per_mtok_usd": 0.60, "output_per_mtok_usd": 2.50, "cache_read_per_mtok_usd": 0.30},
    "fireworks/kimi-k2-thinking": {"input_per_mtok_usd": 0.60, "output_per_mtok_usd": 2.50, "cache_read_per_mtok_usd": 0.30},
    "fireworks/kimi-k2p5": {"input_per_mtok_usd": 0.60, "output_per_mtok_usd": 3.00, "cache_read_per_mtok_usd": 0.10},
    "fireworks/qwen3-vl-30b-a3b-instruct": {"input_per_mtok_usd": 0.15, "output_per_mtok_usd": 0.60, "cache_read_per_mtok_usd": 0.08},
    "fireworks/llama-v3p3-70b-instruct": {"input_per_mtok_usd": 0.90, "output_per_mtok_usd": 0.90, "cache_read_per_mtok_usd": 0.45},
    "fireworks/minimax-m2p1": {"input_per_mtok_usd": 0.30, "output_per_mtok_usd": 1.20, "cache_read_per_mtok_usd": 0.03},
    "fireworks/minimax-m2p5": {"input_per_mtok_usd": 0.30, "output_per_mtok_usd": 1.20, "cache_read_per_mtok_usd": 0.03},
    "fireworks/qwen3-8b": {"input_per_mtok_usd": 0.20, "output_per_mtok_usd": 0.20, "cache_read_per_mtok_usd": 0.10},
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
    source = str(base.get("pricing_source") or _FIREWORKS_PRICING_SOURCE_VERSION)
    prefix = f"FIREWORKS_{purpose.upper()}_"
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
    return with_execution_failures({
        "provider": "fireworks",
        "model": actual_model,
        "pricing_model_family": pricing.get("pricing_model_family", ""),
        "pricing_source": pricing.get("pricing_source", _FIREWORKS_PRICING_SOURCE_VERSION),
        "input_tokens": int(usage.get("input_tokens", 0) or 0),
        "output_tokens": int(usage.get("output_tokens", 0) or 0),
        "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
        "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
        "estimated_cost_usd": _estimate_cost_usd(actual_model, purpose, usage),
    }, usage.get("execution_failures"))


def _supports_strict_schema(model: str) -> bool:
    family = _normalize_model_family(model)
    return model_supports(family, "supports_strict_json_schema") or model_supports(model, "supports_strict_json_schema")


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
        raise RuntimeError("fireworks api key is required")
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else _env_timeout_seconds("FIREWORKS_TIMEOUT_SEC", 90.0)
    attempts = max(1, int(os.getenv("FIREWORKS_RETRY_ATTEMPTS", "3") or "3"))
    base_sleep_sec = _env_timeout_seconds("FIREWORKS_RETRY_BASE_SEC", 0.5)
    return run_chat_json(
        prompt,
        model,
        api_key,
        url=os.getenv("FIREWORKS_API_BASE_URL", "https://api.fireworks.ai/inference/v1/chat/completions"),
        normalize_model_name=_normalize_model_name,
        supports_strict_schema=_supports_strict_schema,
        timeout_sec=req_timeout,
        attempts=attempts,
        base_sleep_sec=base_sleep_sec,
        provider_name="fireworks",
        logger=_log,
        system_instruction=system_instruction,
        max_output_tokens=max_output_tokens,
        response_schema=response_schema,
        schema_name=schema_name,
        include_temperature=True,
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
    text, usage = _chat_json(task["prompt"], model, api_key, system_instruction=task["system"], max_output_tokens=1400, response_schema=task["schema"], schema_name="facts")
    facts = parse_facts_result(text)
    if not facts:
        raise RuntimeError(f"fireworks extract_facts parse failed: response_snippet={text[:500]}")
    return {"facts": facts, "llm": _llm_meta(model, "facts", usage)}


def summarize(title: str | None, facts: list[str], source_text_chars: int | None, model: str, api_key: str) -> dict:
    return wrap_json_transport(
        build_task=lambda: build_summary_task(title, facts, source_text_chars=source_text_chars),
        transport=lambda task: _chat_json(
            task["prompt"],
            model,
            api_key,
            system_instruction=task["system_instruction"],
            max_output_tokens=task["max_tokens"],
            response_schema=task["schema"],
            schema_name="summary",
        ),
        parser=lambda text: finalize_summary_result(text, title=title, facts=facts, source_text_chars=source_text_chars, error_prefix="fireworks summarize parse failed"),
        meta_builder=lambda usage: _llm_meta(model, "summary", usage),
    )


def check_summary_faithfulness(title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
    return run_summary_faithfulness_check(
        title,
        facts,
        summary,
        primary_call=lambda prompt, system_instruction, max_output_tokens=400, response_schema=None, schema_name="summary_faithfulness": _chat_json(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=response_schema,
            schema_name=schema_name,
        ),
        retry_call=lambda prompt, system_instruction, max_output_tokens=240, response_schema=None, schema_name="summary_faithfulness_retry": _chat_json(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=response_schema,
            schema_name=schema_name,
        ),
        prompt_builder=summary_faithfulness_prompt,
        retry_prompt_builder=summary_faithfulness_retry_prompt,
        system_instruction=summary_faithfulness_system_instruction(),
        response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
        meta_builder=lambda usage: _llm_meta(model, "summary", usage),
    )


def check_facts(title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
    return run_facts_check(
        title,
        content,
        facts,
        primary_call=lambda prompt, system_instruction, max_output_tokens=500, response_schema=None, schema_name="facts_check": _chat_json(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=response_schema,
            schema_name=schema_name,
        ),
        retry_call=lambda prompt, system_instruction, max_output_tokens=260, response_schema=None, schema_name="facts_check_retry": _chat_json(
            prompt,
            model,
            api_key,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=response_schema,
            schema_name=schema_name,
        ),
        prompt_builder=facts_check_prompt,
        retry_prompt_builder=facts_check_retry_prompt,
        system_instruction=facts_check_system_instruction(),
        response_schema=FACTS_CHECK_SCHEMA,
        meta_builder=lambda usage: _llm_meta(model, "facts", usage),
    )


def translate_title(title: str, model: str, api_key: str) -> dict:
    translated = _translate_title_to_ja(title, model, api_key)
    return {"translated_title": translated}


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
        timeout_sec=_env_timeout_seconds("FIREWORKS_COMPOSE_DIGEST_TIMEOUT_SEC", 240.0),
    )
    subject, body = parse_digest_result(text, error_prefix="fireworks compose_digest parse failed")
    return {"subject": subject, "body": body, "llm": _llm_meta(model, "digest", usage)}


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
        _log.warning("fireworks compose_digest_cluster_draft primary attempt failed: %s", exc)
        try:
            text, usage = _chat_json(task["fallback_prompt"], model, api_key, max_output_tokens=500, response_schema=None)
        except Exception as retry_exc:
            _log.warning("fireworks compose_digest_cluster_draft fallback failed: %s", retry_exc)
            return {"draft_summary": fallback_cluster_draft_from_source_lines(task["source_lines"]), "llm": _llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}

    draft = parse_cluster_draft_result(text, task["source_lines"])
    return {"draft_summary": draft, "llm": _llm_meta(model, "digest_cluster_draft", usage)}


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
    result = parse_ask_result(text, candidates, error_prefix="fireworks ask missing answer")
    return {**result, "llm": _llm_meta(model, "ask", usage)}


def rank_feed_suggestions(existing_sources: list[dict], preferred_topics: list[str], candidates: list[dict], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
    task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        max_output_tokens=2800,
        response_schema=task["schema"],
        schema_name="rank_feed_suggestions",
    )
    return {"items": parse_rank_feed_result(text, task["candidates"]), "llm": _llm_meta(model, "source_suggestion", usage)}


def suggest_feed_seed_sites(existing_sources: list[dict], preferred_topics: list[str], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
    task = build_seed_sites_task(existing_sources, preferred_topics=preferred_topics, positive_examples=positive_examples, negative_examples=negative_examples)
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        max_output_tokens=2200,
        response_schema=task["schema"],
        schema_name="suggest_feed_seed_sites",
    )
    return {"items": parse_seed_sites_result(text, task["existing_sources"]), "llm": _llm_meta(model, "source_suggestion", usage)}


def localize_facts_to_japanese(title: str | None, facts: list[str], model: str, api_key: str) -> dict:
    task = build_facts_localization_task(title, facts)
    text, usage = _chat_json(
        task["prompt"],
        model,
        api_key,
        system_instruction=task["system_instruction"],
        max_output_tokens=1200,
        response_schema=task["schema"],
        schema_name="localized_facts",
    )
    localized = parse_facts_result(text)
    return {"facts": localized, "llm": _llm_meta(model, "facts", usage)}
