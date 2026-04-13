import asyncio
import logging
import os
import re
from dataclasses import dataclass, field

from app.services.llm_catalog import model_pricing, model_supports
from app.services.llm_text_utils import (
    audio_briefing_script_max_tokens as _audio_briefing_script_max_tokens,
    extract_first_json_object as _extract_first_json_object,
    facts_need_japanese_localization as _facts_need_japanese_localization,
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
    fallback_cluster_draft_from_source_lines,
    build_simple_digest_input,
    parse_cluster_draft_result,
    parse_digest_result,
)
from app.services.feed_task_common import (
    build_ai_navigator_brief_task,
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
    parse_ai_navigator_brief_result,
    parse_audio_briefing_script_result,
    parse_ask_result,
    parse_ask_navigator_result,
    parse_briefing_navigator_result,
    parse_item_navigator_result,
    parse_source_navigator_result,
    parse_rank_feed_result,
    parse_seed_sites_result,
)
from app.services.facts_task_common import build_facts_localization_task, build_facts_task, parse_facts_result
from app.services.openai_compat_transport import run_chat_json, run_chat_json_async
from app.services.task_transport_common import with_execution_failures, wrap_usage_transport


def env_timeout_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        v = float(raw)
        return v if v > 0 else default
    except Exception:
        return default


def env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def normalize_model_name(model: str) -> str:
    return str(model or "").strip()


@dataclass
class ProviderConfig:
    provider_name: str
    env_prefix: str
    pricing_source_version: str
    api_base_url: str
    api_base_url_env: str
    default_model: str = ""
    default_translate_model: str = ""
    legacy_model_pricing: dict | None = None
    model_families: list[str] = field(default_factory=list)
    use_resolve_model_id: bool = False
    use_billed_cost: bool = False
    include_resolved_model_in_meta: bool = False
    facts_output_mode: str = "object"
    facts_pass_schema: bool = True
    supports_response_format: bool = True
    no_temperature_families: list[str] = field(default_factory=list)


class OpenAICompatProvider:
    def __init__(self, config: ProviderConfig):
        self.config = config
        self._log = logging.getLogger(f"app.services.{config.provider_name}_service")

    def _normalize_model_name(self, model: str) -> str:
        if self.config.use_resolve_model_id:
            from app.services.llm_catalog import resolve_model_id
            return resolve_model_id(str(model or "").strip())
        return str(model or "").strip()

    def _normalize_model_family(self, model: str) -> str:
        m = self._normalize_model_name(model)
        if self.config.legacy_model_pricing:
            if model_pricing(m) is not None:
                return m
            for family in sorted(self.config.legacy_model_pricing.keys(), key=len, reverse=True):
                if m == family or m.startswith(family + "-"):
                    return family
            return m
        if self.config.model_families:
            if model_pricing(m) is not None:
                return m
            for family in self.config.model_families:
                if m == family or m.startswith(family + "-"):
                    return family
        return m

    def _supports_strict_schema(self, model: str) -> bool:
        family = self._normalize_model_family(model)
        return model_supports(family, "supports_strict_json_schema") or model_supports(model, "supports_strict_json_schema")

    def _pricing_for_model(self, model: str, purpose: str) -> dict:
        family = self._normalize_model_family(model)
        catalog_pricing = model_pricing(family) or model_pricing(model)
        if self.config.legacy_model_pricing:
            base = dict(catalog_pricing or self.config.legacy_model_pricing.get(family, {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0}))
        else:
            base = dict(catalog_pricing or {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0})
        source = str(base.get("pricing_source") or self.config.pricing_source_version)
        prefix = f"{self.config.env_prefix}_{purpose.upper()}_"
        override_map = {
            "input_per_mtok_usd": env_optional_float(prefix + "INPUT_PER_MTOK_USD"),
            "output_per_mtok_usd": env_optional_float(prefix + "OUTPUT_PER_MTOK_USD"),
            "cache_read_per_mtok_usd": env_optional_float(prefix + "CACHE_READ_PER_MTOK_USD"),
        }
        for k, v in override_map.items():
            if v is not None:
                base[k] = v
                source = "env_override"
        base["pricing_source"] = source
        base["pricing_model_family"] = family
        return base

    def _estimate_cost_usd(self, model: str, purpose: str, usage: dict) -> float:
        p = self._pricing_for_model(model, purpose)
        non_cached_input_tokens = max(0, int(usage.get("input_tokens", 0) or 0) - int(usage.get("cache_read_input_tokens", 0) or 0))
        total = 0.0
        total += non_cached_input_tokens / 1_000_000 * p["input_per_mtok_usd"]
        total += int(usage.get("output_tokens", 0) or 0) / 1_000_000 * p["output_per_mtok_usd"]
        total += int(usage.get("cache_read_input_tokens", 0) or 0) / 1_000_000 * p.get("cache_read_per_mtok_usd", 0.0)
        return round(total, 8)

    def _llm_meta(self, model: str, purpose: str, usage: dict) -> dict:
        if self.config.include_resolved_model_in_meta:
            return self._llm_meta_resolved(model, purpose, usage)
        pricing = self._pricing_for_model(model, purpose)
        actual_model = self._normalize_model_name(model)
        return with_execution_failures({
            "provider": self.config.provider_name,
            "model": actual_model,
            "pricing_model_family": pricing.get("pricing_model_family", ""),
            "pricing_source": pricing.get("pricing_source", self.config.pricing_source_version),
            "input_tokens": int(usage.get("input_tokens", 0) or 0),
            "output_tokens": int(usage.get("output_tokens", 0) or 0),
            "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
            "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
            "estimated_cost_usd": self._resolve_cost(actual_model, purpose, usage),
        }, usage.get("execution_failures"))

    def _llm_meta_resolved(self, model: str, purpose: str, usage: dict) -> dict:
        requested_model = str(usage.get("requested_model") or model or "").strip()
        resolved_model = str(usage.get("resolved_model") or "").strip()
        pricing_target = resolved_model or requested_model or model
        pricing = self._pricing_for_model(pricing_target, purpose)
        actual_model = self._normalize_model_name(pricing_target)
        return with_execution_failures({
            "provider": self.config.provider_name,
            "model": requested_model or model,
            "requested_model": requested_model or model,
            "resolved_model": resolved_model,
            "pricing_model_family": pricing.get("pricing_model_family", actual_model),
            "pricing_source": pricing.get("pricing_source", self.config.pricing_source_version),
            "input_tokens": int(usage.get("input_tokens", 0) or 0),
            "output_tokens": int(usage.get("output_tokens", 0) or 0),
            "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
            "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
            "estimated_cost_usd": self._resolve_cost(actual_model, purpose, usage),
        }, usage.get("execution_failures"))

    def _resolve_cost(self, model: str, purpose: str, usage: dict) -> float:
        if self.config.use_billed_cost and usage.get("billed_cost_usd") is not None:
            return float(usage["billed_cost_usd"])
        return self._estimate_cost_usd(model, purpose, usage)

    def _get_chat_url(self) -> str:
        return os.getenv(self.config.api_base_url_env, self.config.api_base_url)

    def _include_temperature(self, model: str) -> bool:
        if self.config.no_temperature_families:
            return self._normalize_model_family(model) not in self.config.no_temperature_families
        return True

    def _normalize_temperature(self, model: str, value: float | None) -> float | None:
        return value

    def _normalize_top_p(self, value: float | None) -> float | None:
        return value

    def _post_process_chat_result(self, text: str, usage: dict, model: str, api_key: str, response_schema, timeout) -> tuple[str, dict]:
        return text, usage

    async def _post_process_chat_result_async(self, text: str, usage: dict, model: str, api_key: str, response_schema, timeout) -> tuple[str, dict]:
        return self._post_process_chat_result(text, usage, model, api_key, response_schema, timeout)

    def _request_response_schema(self, response_schema: dict | None) -> dict | None:
        if not self.config.supports_response_format:
            return None
        return response_schema

    def _chat_json(
        self,
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
            raise RuntimeError(f"{self.config.provider_name} api key is required")
        req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else env_timeout_seconds(f"{self.config.env_prefix}_TIMEOUT_SEC", 300.0)
        attempts = max(1, int(os.getenv(f"{self.config.env_prefix}_RETRY_ATTEMPTS", "3") or "3"))
        base_sleep_sec = env_timeout_seconds(f"{self.config.env_prefix}_RETRY_BASE_SEC", 0.5)
        request_response_schema = self._request_response_schema(response_schema)
        text, usage = run_chat_json(
            prompt,
            model,
            api_key,
            url=self._get_chat_url(),
            normalize_model_name=self._normalize_model_name,
            supports_strict_schema=self._supports_strict_schema,
            timeout_sec=req_timeout,
            attempts=attempts,
            base_sleep_sec=base_sleep_sec,
            provider_name=self.config.provider_name,
            logger=self._log,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=request_response_schema,
            schema_name=schema_name,
            include_temperature=self._include_temperature(model),
            temperature=self._normalize_temperature(model, temperature),
            top_p=self._normalize_top_p(top_p),
        )
        return self._post_process_chat_result(text, usage, model, api_key, response_schema, req_timeout)

    async def _chat_json_async(
        self,
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
            raise RuntimeError(f"{self.config.provider_name} api key is required")
        req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else env_timeout_seconds(f"{self.config.env_prefix}_TIMEOUT_SEC", 300.0)
        attempts = max(1, int(os.getenv(f"{self.config.env_prefix}_RETRY_ATTEMPTS", "3") or "3"))
        base_sleep_sec = env_timeout_seconds(f"{self.config.env_prefix}_RETRY_BASE_SEC", 0.5)
        request_response_schema = self._request_response_schema(response_schema)
        text, usage = await run_chat_json_async(
            prompt,
            model,
            api_key,
            url=self._get_chat_url(),
            normalize_model_name=self._normalize_model_name,
            supports_strict_schema=self._supports_strict_schema,
            timeout_sec=req_timeout,
            attempts=attempts,
            base_sleep_sec=base_sleep_sec,
            provider_name=self.config.provider_name,
            logger=self._log,
            system_instruction=system_instruction,
            max_output_tokens=max_output_tokens,
            response_schema=request_response_schema,
            schema_name=schema_name,
            include_temperature=self._include_temperature(model),
            temperature=self._normalize_temperature(model, temperature),
            top_p=self._normalize_top_p(top_p),
        )
        return await self._post_process_chat_result_async(text, usage, model, api_key, response_schema, req_timeout)

    def _translate_title_to_ja(self, title: str, model: str, api_key: str) -> str:
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
  "translated_title": "日本語訳"
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
                    self._chat_json(
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
            plain_retry_call=lambda: self._chat_json(plain_prompt, model, api_key, max_output_tokens=120)[0],
        )

    def extract_facts(self, title: str | None, content: str, model: str, api_key: str) -> dict:
        task = build_facts_task(title, content, output_mode=self.config.facts_output_mode)
        chat_kwargs: dict = dict(
            system_instruction=task["system_instruction"],
            max_output_tokens=1500,
        )
        if self.config.facts_pass_schema:
            chat_kwargs["response_schema"] = task["schema"]
            chat_kwargs["schema_name"] = "facts"
        text, usage = self._chat_json(task["prompt"], model, api_key, **chat_kwargs)
        facts = parse_facts_result(text)
        localization_llm = None
        if not facts:
            raise RuntimeError(f"{self.config.provider_name} extract_facts parse failed: response_snippet={text[:500]}")
        if _facts_need_japanese_localization(facts):
            localize_task = build_facts_localization_task(title, facts)
            localized_text, localized_usage = self._chat_json(
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
                localization_llm = self._llm_meta(model, "facts_localization", localized_usage)
        return {"facts": facts, "llm": self._llm_meta(model, "facts", usage), "facts_localization_llm": localization_llm}

    def summarize(self, title: str | None, facts: list[str], source_text_chars: int | None = None, model: str = "", api_key: str = "") -> dict:
        if not model:
            model = self.config.default_model
        task = build_summary_task(title, facts, source_text_chars)
        text, usage = self._chat_json(
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
            translate_func=lambda raw_title: self._translate_title_to_ja(raw_title, model, api_key),
            llm=self._llm_meta(model, "summary", usage),
            error_prefix=f"{self.config.provider_name} summarize parse failed",
            response_text=text,
        )

    def check_summary_faithfulness(self, title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
        return run_summary_faithfulness_check(
            lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    summary_faithfulness_prompt(title, facts, summary),
                    model,
                    api_key,
                    system_instruction=summary_faithfulness_system_instruction(),
                    max_output_tokens=320,
                    response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
                    schema_name="summary_faithfulness",
                ),
                lambda usage: self._llm_meta(model, "faithfulness_check", usage),
            ),
            retry_call=lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    summary_faithfulness_retry_prompt(title, facts, summary),
                    model,
                    api_key,
                    system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                    max_output_tokens=120,
                    response_schema=None,
                ),
                lambda usage: self._llm_meta(model, "faithfulness_check", usage),
            ),
        )

    def check_facts(self, title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
        return run_facts_check(
            lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    facts_check_prompt(title, content, facts),
                    model,
                    api_key,
                    system_instruction=facts_check_system_instruction(),
                    max_output_tokens=320,
                    response_schema=FACTS_CHECK_SCHEMA,
                    schema_name="facts_check",
                ),
                lambda usage: self._llm_meta(model, "facts_check", usage),
            ),
            retry_call=lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    facts_check_retry_prompt(title, content, facts),
                    model,
                    api_key,
                    system_instruction=facts_check_system_instruction(),
                    max_output_tokens=220,
                    response_schema=FACTS_CHECK_SCHEMA,
                    schema_name="facts_check",
                ),
                lambda usage: self._llm_meta(model, "facts_check", usage),
            ),
        )

    def translate_title(self, title: str, model: str = "", api_key: str = "") -> dict:
        if not model:
            model = self.config.default_translate_model
        src = (title or "").strip()
        if not src:
            return {"translated_title": "", "llm": None}
        translated = self._translate_title_to_ja(src, model, api_key)
        return {"translated_title": translated[:300], "llm": None}

    def compose_digest(self, digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
        if not items:
            return {
                "subject": f"Sifto Digest - {digest_date}",
                "body": "本日のダイジェスト対象記事はありませんでした。",
                "llm": self._llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
            }
        task = build_digest_task(digest_date, len(items), build_simple_digest_input(items))
        text, usage = self._chat_json(
            task["prompt"],
            model,
            api_key,
            system_instruction=task["system_instruction"],
            max_output_tokens=8000,
            response_schema=task["schema"],
            schema_name="digest",
            timeout_sec=env_timeout_seconds(f"{self.config.env_prefix}_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0),
        )
        subject, body = parse_digest_result(text, error_prefix=f"{self.config.provider_name} compose_digest parse failed")
        return {"subject": subject, "body": body, "llm": self._llm_meta(model, "digest", usage)}

    def ask_question(self, query: str, candidates: list[dict], model: str, api_key: str) -> dict:
        if not candidates:
            return {
                "answer": "該当する記事は見つかりませんでした。",
                "bullets": [],
                "citations": [],
                "llm": self._llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0}),
            }
        task = build_ask_task(query, candidates)
        text, usage = self._chat_json(
            task["prompt"],
            model,
            api_key,
            system_instruction=task["system_instruction"],
            max_output_tokens=3200,
            response_schema=task["schema"],
            schema_name="ask",
        )
        result = parse_ask_result(text, candidates, error_prefix=f"{self.config.provider_name} ask missing answer")
        return {**result, "llm": self._llm_meta(model, "ask", usage)}

    def compose_digest_cluster_draft(self, cluster_label: str, item_count: int, topics: list[str], source_lines: list[str], model: str, api_key: str) -> dict:
        task = build_cluster_draft_task(str(cluster_label or "話題").strip() or "話題", item_count, topics, source_lines)
        if not task["source_lines"]:
            return {"draft_summary": "", "llm": self._llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}
        try:
            text, usage = self._chat_json(
                task["prompt"],
                model,
                api_key,
                system_instruction=task["system_instruction"],
                max_output_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
                response_schema=task["schema"],
                schema_name="digest_cluster_draft",
            )
        except Exception as exc:
            self._log.warning("%s compose_digest_cluster_draft primary attempt failed: %s", self.config.provider_name, exc)
            try:
                text, usage = self._chat_json(task["fallback_prompt"], model, api_key, max_output_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS, response_schema=None)
            except Exception as retry_exc:
                self._log.warning("%s compose_digest_cluster_draft fallback failed: %s", self.config.provider_name, retry_exc)
                return {"draft_summary": fallback_cluster_draft_from_source_lines(task["source_lines"]), "llm": self._llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}

        draft = parse_cluster_draft_result(text, task["source_lines"])
        return {"draft_summary": draft, "llm": self._llm_meta(model, "digest_cluster_draft", usage)}

    def rank_feed_suggestions(self, existing_sources: list[dict], preferred_topics: list[str], candidates: list[dict], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
        task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=2800, response_schema=task["schema"], schema_name="rank_feed_suggestions")
        return {"items": parse_rank_feed_result(text, task["candidates"]), "llm": self._llm_meta(model, "source_suggestion", usage)}

    def generate_briefing_navigator(self, persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
        task = build_briefing_navigator_task(persona, candidates, intro_context)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=1800, response_schema=task["schema"], schema_name="briefing_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_briefing_navigator_result(text, task["candidates"])
        return {"intro": out["intro"], "picks": out["picks"], "llm": self._llm_meta(model, "briefing_navigator", usage)}

    def compose_ai_navigator_brief(self, persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
        task = build_ai_navigator_brief_task(persona, candidates, intro_context)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=3200, response_schema=task["schema"], schema_name="ai_navigator_brief", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_ai_navigator_brief_result(text, task["candidates"], intro_context)
        return {"title": out["title"], "intro": out["intro"], "summary": out["summary"], "ending": out["ending"], "items": out["items"], "llm": self._llm_meta(model, "ai_navigator_brief", usage)}

    def generate_item_navigator(self, persona: str, article: dict, model: str, api_key: str) -> dict:
        task = build_item_navigator_task(persona, article)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="item_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_item_navigator_result(text, task["article"])
        return {"headline": out["headline"], "commentary": out["commentary"], "stance_tags": out["stance_tags"], "llm": self._llm_meta(model, "item_navigator", usage)}

    def generate_audio_briefing_script(
        self,
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
        text, usage = self._chat_json(
            task["prompt"],
            model,
            api_key,
            max_output_tokens=_audio_briefing_script_max_tokens(task["target_chars"], str((intro_context or {}).get("audio_briefing_conversation_mode") or "single")),
            response_schema=task["schema"],
            schema_name="audio_briefing_script",
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
            "llm": self._llm_meta(model, "audio_briefing_script", usage),
        }

    def generate_ask_navigator(self, persona: str, ask_input: dict, model: str, api_key: str) -> dict:
        task = build_ask_navigator_task(persona, ask_input)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=2400, response_schema=task["schema"], schema_name="ask_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_ask_navigator_result(text, task["input"])
        return {"headline": out["headline"], "commentary": out["commentary"], "next_angles": out["next_angles"], "llm": self._llm_meta(model, "ask_navigator", usage)}

    def generate_source_navigator(self, persona: str, candidates: list[dict], model: str, api_key: str) -> dict:
        task = build_source_navigator_task(persona, candidates)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=2600, response_schema=task["schema"], schema_name="source_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_source_navigator_result(text, task["candidates"])
        return {"overview": out["overview"], "keep": out["keep"], "watch": out["watch"], "standout": out["standout"], "llm": self._llm_meta(model, "source_navigator", usage)}

    def suggest_feed_seed_sites(self, existing_sources: list[dict], preferred_topics: list[str], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
        task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
        text, usage = self._chat_json(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="suggest_feed_seed_sites")
        out = parse_seed_sites_result(text, task["existing_sources"])
        if len(out) == 0:
            try:
                rescue_text, rescue_usage = self._chat_json(
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
                self._log.warning("%s suggest_feed_seed_sites rescue failed: %s", self.config.provider_name, exc)
        return {"items": out, "llm": self._llm_meta(model, "source_suggestion", usage)}

    async def extract_facts_async(self, title: str | None, content: str, model: str, api_key: str) -> dict:
        task = build_facts_task(title, content, output_mode=self.config.facts_output_mode)
        chat_kwargs: dict = dict(
            system_instruction=task["system_instruction"],
            max_output_tokens=1500,
        )
        if self.config.facts_pass_schema:
            chat_kwargs["response_schema"] = task["schema"]
            chat_kwargs["schema_name"] = "facts"
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, **chat_kwargs)
        facts = parse_facts_result(text)
        localization_llm = None
        if not facts:
            raise RuntimeError(f"{self.config.provider_name} extract_facts parse failed: response_snippet={text[:500]}")
        if _facts_need_japanese_localization(facts):
            localize_task = build_facts_localization_task(title, facts)
            localized_text, localized_usage = await self._chat_json_async(
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
                localization_llm = self._llm_meta(model, "facts_localization", localized_usage)
        return {"facts": facts, "llm": self._llm_meta(model, "facts", usage), "facts_localization_llm": localization_llm}

    async def summarize_async(self, title: str | None, facts: list[str], source_text_chars: int | None = None, model: str = "", api_key: str = "") -> dict:
        if not model:
            model = self.config.default_model
        task = build_summary_task(title, facts, source_text_chars)
        text, usage = await self._chat_json_async(
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
        return await asyncio.to_thread(
            finalize_summary_result,
            title=title,
            summary_text=str(data.get("summary") or "").strip(),
            topics=topics,
            raw_score_breakdown=data.get("score_breakdown"),
            score_reason=str(data.get("score_reason") or "").strip(),
            translated_title=str(data.get("translated_title") or "").strip(),
            translate_func=lambda raw_title: self._translate_title_to_ja(raw_title, model, api_key),
            llm=self._llm_meta(model, "summary", usage),
            error_prefix=f"{self.config.provider_name} summarize parse failed",
            response_text=text,
        )

    async def check_summary_faithfulness_async(self, title: str | None, facts: list[str], summary: str, model: str, api_key: str) -> dict:
        return await asyncio.to_thread(
            run_summary_faithfulness_check,
            lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    summary_faithfulness_prompt(title, facts, summary),
                    model,
                    api_key,
                    system_instruction=summary_faithfulness_system_instruction(),
                    max_output_tokens=320,
                    response_schema=SUMMARY_FAITHFULNESS_SCHEMA,
                    schema_name="summary_faithfulness",
                ),
                lambda usage: self._llm_meta(model, "faithfulness_check", usage),
            ),
            retry_call=lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    summary_faithfulness_retry_prompt(title, facts, summary),
                    model,
                    api_key,
                    system_instruction="pass / warn / fail のいずれか1語のみを返す。",
                    max_output_tokens=120,
                    response_schema=None,
                ),
                lambda usage: self._llm_meta(model, "faithfulness_check", usage),
            ),
        )

    async def check_facts_async(self, title: str | None, content: str, facts: list[str], model: str, api_key: str) -> dict:
        return await asyncio.to_thread(
            run_facts_check,
            lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    facts_check_prompt(title, content, facts),
                    model,
                    api_key,
                    system_instruction=facts_check_system_instruction(),
                    max_output_tokens=320,
                    response_schema=FACTS_CHECK_SCHEMA,
                    schema_name="facts_check",
                ),
                lambda usage: self._llm_meta(model, "facts_check", usage),
            ),
            retry_call=lambda: wrap_usage_transport(
                lambda: self._chat_json(
                    facts_check_retry_prompt(title, content, facts),
                    model,
                    api_key,
                    system_instruction=facts_check_system_instruction(),
                    max_output_tokens=220,
                    response_schema=FACTS_CHECK_SCHEMA,
                    schema_name="facts_check",
                ),
                lambda usage: self._llm_meta(model, "facts_check", usage),
            ),
        )

    async def translate_title_async(self, title: str, model: str = "", api_key: str = "") -> dict:
        if not model:
            model = self.config.default_translate_model
        src = (title or "").strip()
        if not src:
            return {"translated_title": "", "llm": None}
        translated = await asyncio.to_thread(self._translate_title_to_ja, src, model, api_key)
        return {"translated_title": translated[:300], "llm": None}

    async def compose_digest_async(self, digest_date: str, items: list[dict], model: str, api_key: str) -> dict:
        if not items:
            return {
                "subject": f"Sifto Digest - {digest_date}",
                "body": "本日のダイジェスト対象記事はありませんでした。",
                "llm": self._llm_meta(model, "digest", {"input_tokens": 0, "output_tokens": 0}),
            }
        task = build_digest_task(digest_date, len(items), build_simple_digest_input(items))
        text, usage = await self._chat_json_async(
            task["prompt"],
            model,
            api_key,
            system_instruction=task["system_instruction"],
            max_output_tokens=8000,
            response_schema=task["schema"],
            schema_name="digest",
            timeout_sec=env_timeout_seconds(f"{self.config.env_prefix}_COMPOSE_DIGEST_TIMEOUT_SEC", 300.0),
        )
        subject, body = parse_digest_result(text, error_prefix=f"{self.config.provider_name} compose_digest parse failed")
        return {"subject": subject, "body": body, "llm": self._llm_meta(model, "digest", usage)}

    async def ask_question_async(self, query: str, candidates: list[dict], model: str, api_key: str) -> dict:
        if not candidates:
            return {
                "answer": "該当する記事は見つかりませんでした。",
                "bullets": [],
                "citations": [],
                "llm": self._llm_meta(model, "ask", {"input_tokens": 0, "output_tokens": 0}),
            }
        task = build_ask_task(query, candidates)
        text, usage = await self._chat_json_async(
            task["prompt"],
            model,
            api_key,
            system_instruction=task["system_instruction"],
            max_output_tokens=3200,
            response_schema=task["schema"],
            schema_name="ask",
        )
        result = parse_ask_result(text, candidates, error_prefix=f"{self.config.provider_name} ask missing answer")
        return {**result, "llm": self._llm_meta(model, "ask", usage)}

    async def compose_digest_cluster_draft_async(self, cluster_label: str, item_count: int, topics: list[str], source_lines: list[str], model: str, api_key: str) -> dict:
        task = build_cluster_draft_task(str(cluster_label or "話題").strip() or "話題", item_count, topics, source_lines)
        if not task["source_lines"]:
            return {"draft_summary": "", "llm": self._llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}
        try:
            text, usage = await self._chat_json_async(
                task["prompt"],
                model,
                api_key,
                system_instruction=task["system_instruction"],
                max_output_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS,
                response_schema=task["schema"],
                schema_name="digest_cluster_draft",
            )
        except Exception as exc:
            self._log.warning("%s compose_digest_cluster_draft primary attempt failed: %s", self.config.provider_name, exc)
            try:
                text, usage = await self._chat_json_async(task["fallback_prompt"], model, api_key, max_output_tokens=DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS, response_schema=None)
            except Exception as retry_exc:
                self._log.warning("%s compose_digest_cluster_draft fallback failed: %s", self.config.provider_name, retry_exc)
                return {"draft_summary": fallback_cluster_draft_from_source_lines(task["source_lines"]), "llm": self._llm_meta(model, "digest_cluster_draft", {"input_tokens": 0, "output_tokens": 0})}

        draft = parse_cluster_draft_result(text, task["source_lines"])
        return {"draft_summary": draft, "llm": self._llm_meta(model, "digest_cluster_draft", usage)}

    async def rank_feed_suggestions_async(self, existing_sources: list[dict], preferred_topics: list[str], candidates: list[dict], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
        task = build_rank_feed_task(existing_sources, preferred_topics, candidates, positive_examples, negative_examples)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=2800, response_schema=task["schema"], schema_name="rank_feed_suggestions")
        return {"items": parse_rank_feed_result(text, task["candidates"]), "llm": self._llm_meta(model, "source_suggestion", usage)}

    async def generate_briefing_navigator_async(self, persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
        task = build_briefing_navigator_task(persona, candidates, intro_context)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=1800, response_schema=task["schema"], schema_name="briefing_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_briefing_navigator_result(text, task["candidates"])
        return {"intro": out["intro"], "picks": out["picks"], "llm": self._llm_meta(model, "briefing_navigator", usage)}

    async def compose_ai_navigator_brief_async(self, persona: str, candidates: list[dict], intro_context: dict, model: str, api_key: str) -> dict:
        task = build_ai_navigator_brief_task(persona, candidates, intro_context)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=3200, response_schema=task["schema"], schema_name="ai_navigator_brief", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_ai_navigator_brief_result(text, task["candidates"], intro_context)
        return {"title": out["title"], "intro": out["intro"], "summary": out["summary"], "ending": out["ending"], "items": out["items"], "llm": self._llm_meta(model, "ai_navigator_brief", usage)}

    async def generate_item_navigator_async(self, persona: str, article: dict, model: str, api_key: str) -> dict:
        task = build_item_navigator_task(persona, article)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="item_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_item_navigator_result(text, task["article"])
        return {"headline": out["headline"], "commentary": out["commentary"], "stance_tags": out["stance_tags"], "llm": self._llm_meta(model, "item_navigator", usage)}

    async def generate_audio_briefing_script_async(
        self,
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
        text, usage = await self._chat_json_async(
            task["prompt"],
            model,
            api_key,
            max_output_tokens=_audio_briefing_script_max_tokens(task["target_chars"], str((intro_context or {}).get("audio_briefing_conversation_mode") or "single")),
            response_schema=task["schema"],
            schema_name="audio_briefing_script",
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
            "llm": self._llm_meta(model, "audio_briefing_script", usage),
        }

    async def generate_ask_navigator_async(self, persona: str, ask_input: dict, model: str, api_key: str) -> dict:
        task = build_ask_navigator_task(persona, ask_input)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=2400, response_schema=task["schema"], schema_name="ask_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_ask_navigator_result(text, task["input"])
        return {"headline": out["headline"], "commentary": out["commentary"], "next_angles": out["next_angles"], "llm": self._llm_meta(model, "ask_navigator", usage)}

    async def generate_source_navigator_async(self, persona: str, candidates: list[dict], model: str, api_key: str) -> dict:
        task = build_source_navigator_task(persona, candidates)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=2600, response_schema=task["schema"], schema_name="source_navigator", temperature=task["sampling_profile"]["temperature"], top_p=task["sampling_profile"]["top_p"])
        out = parse_source_navigator_result(text, task["candidates"])
        return {"overview": out["overview"], "keep": out["keep"], "watch": out["watch"], "standout": out["standout"], "llm": self._llm_meta(model, "source_navigator", usage)}

    async def suggest_feed_seed_sites_async(self, existing_sources: list[dict], preferred_topics: list[str], positive_examples: list[dict] | None, negative_examples: list[dict] | None, model: str, api_key: str) -> dict:
        task = build_seed_sites_task(existing_sources, preferred_topics, positive_examples, negative_examples)
        text, usage = await self._chat_json_async(task["prompt"], model, api_key, max_output_tokens=2200, response_schema=task["schema"], schema_name="suggest_feed_seed_sites")
        out = parse_seed_sites_result(text, task["existing_sources"])
        if len(out) == 0:
            try:
                rescue_text, rescue_usage = await self._chat_json_async(
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
                self._log.warning("%s suggest_feed_seed_sites rescue failed: %s", self.config.provider_name, exc)
        return {"items": out, "llm": self._llm_meta(model, "source_suggestion", usage)}
