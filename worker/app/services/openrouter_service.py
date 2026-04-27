import os
import re

import httpx

from .openai_compat_transport import run_chat_json
from .provider_base import ProviderConfig, OpenAICompatProvider, env_timeout_seconds
from .llm_text_utils import extract_first_json_object as _extract_first_json_object


def _load_repair_json():
    try:
        from json_repair import repair_json
    except Exception:
        return None
    return repair_json


def _should_repair_structured_json(model: str, response_schema: dict | None) -> bool:
    return response_schema is not None


def _repair_structured_json_text(text: str, model: str, response_schema: dict | None) -> str:
    cleaned = str(text or "").strip()
    if not cleaned or not _should_repair_structured_json(model, response_schema):
        return cleaned
    if _extract_first_json_object(cleaned) is not None:
        return cleaned
    repair_json = _load_repair_json()
    if repair_json is None:
        return cleaned
    try:
        repaired = str(repair_json(cleaned, skip_json_loads=True, ensure_ascii=False) or "").strip()
    except Exception:
        return cleaned
    if repaired and _extract_first_json_object(repaired) is not None:
        return repaired
    return cleaned


def _canonicalize_resolved_model(model: str, provider) -> str:
    from .llm_catalog import model_pricing
    normalized = provider._normalize_model_name(model)
    if model_pricing(normalized) is not None:
        return normalized
    anthropic_match = re.fullmatch(r"anthropic/claude-(?P<version>\d+(?:\.\d+)*)-(?P<tier>opus|sonnet|haiku)-\d{8}", normalized)
    if anthropic_match:
        version = anthropic_match.group("version")
        tier = anthropic_match.group("tier")
        return f"anthropic/claude-{tier}-{version}"
    return normalized


class _OpenRouterProvider(OpenAICompatProvider):
    def _normalize_model_family(self, model: str) -> str:
        return _canonicalize_resolved_model(model, self)

    def _generation_url(self) -> str:
        base = os.getenv("OPENROUTER_GENERATION_API_URL", "").strip()
        if base:
            return base
        api_base = os.getenv("OPENROUTER_API_BASE_URL", "https://openrouter.ai/api/v1/chat/completions").strip()
        if api_base.endswith("/chat/completions"):
            return api_base[: -len("/chat/completions")] + "/generation"
        return "https://openrouter.ai/api/v1/generation"

    def _fetch_generation_cost_details(self, api_key: str, generation_id: str, timeout_sec: float) -> dict:
        generation_id = str(generation_id or "").strip()
        if not api_key or not generation_id:
            return {}
        try:
            with httpx.Client(timeout=timeout_sec) as client:
                resp = client.get(
                    self._generation_url(),
                    headers={"Authorization": f"Bearer {api_key}"},
                    params={"id": generation_id},
                )
        except Exception as err:
            self._log.warning("openrouter generation lookup failed id=%s err=%s", generation_id, err)
            return {}
        if resp.status_code >= 400:
            self._log.warning("openrouter generation lookup failed id=%s status=%s body=%s", generation_id, resp.status_code, resp.text[:400])
            return {}
        try:
            data = resp.json() if resp.content else {}
        except Exception:
            return {}
        result: dict = {}
        total_cost = data.get("data", {}).get("total_cost")
        if total_cost is None:
            total_cost = data.get("total_cost")
        if total_cost is not None:
            try:
                result["billed_cost_usd"] = float(total_cost)
            except Exception:
                pass
        resolved_model = str(data.get("data", {}).get("model") or data.get("model") or "").strip()
        if resolved_model:
            result["resolved_model"] = resolved_model
        return result

    async def _fetch_generation_cost_details_async(self, api_key: str, generation_id: str, timeout_sec: float) -> dict:
        generation_id = str(generation_id or "").strip()
        if not api_key or not generation_id:
            return {}
        try:
            async with httpx.AsyncClient(timeout=timeout_sec) as client:
                resp = await client.get(
                    self._generation_url(),
                    headers={"Authorization": f"Bearer {api_key}"},
                    params={"id": generation_id},
                )
        except Exception as err:
            self._log.warning("openrouter generation lookup failed id=%s err=%s", generation_id, err)
            return {}
        if resp.status_code >= 400:
            self._log.warning("openrouter generation lookup failed id=%s status=%s body=%s", generation_id, resp.status_code, resp.text[:400])
            return {}
        try:
            data = resp.json() if resp.content else {}
        except Exception:
            return {}
        result: dict = {}
        total_cost = data.get("data", {}).get("total_cost")
        if total_cost is None:
            total_cost = data.get("total_cost")
        if total_cost is not None:
            try:
                result["billed_cost_usd"] = float(total_cost)
            except Exception:
                pass
        resolved_model = str(data.get("data", {}).get("model") or data.get("model") or "").strip()
        if resolved_model:
            result["resolved_model"] = resolved_model
        return result

    def _post_process_chat_result(self, text, usage, model, api_key, response_schema, timeout):
        if usage.get("billed_cost_usd") is None and usage.get("generation_id"):
            generation_meta = self._fetch_generation_cost_details(api_key, str(usage.get("generation_id") or ""), timeout)
            if generation_meta.get("billed_cost_usd") is not None:
                usage["billed_cost_usd"] = generation_meta["billed_cost_usd"]
            if generation_meta.get("resolved_model") and not usage.get("resolved_model"):
                usage["resolved_model"] = generation_meta["resolved_model"]
        return _repair_structured_json_text(text, model, response_schema), usage

    async def _post_process_chat_result_async(self, text, usage, model, api_key, response_schema, timeout):
        if usage.get("billed_cost_usd") is None and usage.get("generation_id"):
            generation_meta = await self._fetch_generation_cost_details_async(api_key, str(usage.get("generation_id") or ""), timeout)
            if generation_meta.get("billed_cost_usd") is not None:
                usage["billed_cost_usd"] = generation_meta["billed_cost_usd"]
            if generation_meta.get("resolved_model") and not usage.get("resolved_model"):
                usage["resolved_model"] = generation_meta["resolved_model"]
        return _repair_structured_json_text(text, model, response_schema), usage


_config = ProviderConfig(
    provider_name="openrouter",
    env_prefix="OPENROUTER",
    pricing_source_version="openrouter_snapshot",
    api_base_url="https://openrouter.ai/api/v1/chat/completions",
    api_base_url_env="OPENROUTER_API_BASE_URL",
    use_resolve_model_id=True,
    use_billed_cost=True,
    include_resolved_model_in_meta=True,
)
_p = _OpenRouterProvider(_config)
_llm_meta = _p._llm_meta


def _fetch_generation_cost_details(api_key: str, generation_id: str, timeout_sec: float) -> dict:
    return _p._fetch_generation_cost_details(api_key, generation_id, timeout_sec)


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
        raise RuntimeError(f"{_config.provider_name} api key is required")
    req_timeout = timeout_sec if timeout_sec and timeout_sec > 0 else env_timeout_seconds(f"{_config.env_prefix}_TIMEOUT_SEC", 90.0)
    attempts = max(1, int(os.getenv(f"{_config.env_prefix}_RETRY_ATTEMPTS", "3") or "3"))
    base_sleep_sec = env_timeout_seconds(f"{_config.env_prefix}_RETRY_BASE_SEC", 0.5)
    text, usage = run_chat_json(
        prompt,
        model,
        api_key,
        url=_p._get_chat_url(),
        normalize_model_name=_p._normalize_model_name,
        supports_strict_schema=_p._supports_strict_schema,
        timeout_sec=req_timeout,
        attempts=attempts,
        base_sleep_sec=base_sleep_sec,
        provider_name=_config.provider_name,
        logger=_p._log,
        system_instruction=system_instruction,
        max_output_tokens=max_output_tokens,
        response_schema=response_schema,
        schema_name=schema_name,
        include_temperature=_p._include_temperature(model),
        temperature=_p._normalize_temperature(model, temperature),
        top_p=_p._normalize_top_p(model, top_p),
    )
    if usage.get("billed_cost_usd") is None and usage.get("generation_id"):
        generation_meta = _fetch_generation_cost_details(api_key, str(usage.get("generation_id") or ""), req_timeout)
        if generation_meta.get("billed_cost_usd") is not None:
            usage["billed_cost_usd"] = generation_meta["billed_cost_usd"]
        if generation_meta.get("resolved_model") and not usage.get("resolved_model"):
            usage["resolved_model"] = generation_meta["resolved_model"]
    return _repair_structured_json_text(text, model, response_schema), usage

extract_facts = _p.extract_facts
summarize = _p.summarize
check_summary_faithfulness = _p.check_summary_faithfulness
check_facts = _p.check_facts
translate_title = _p.translate_title
compose_digest = _p.compose_digest
ask_question = _p.ask_question
ask_rerank = _p.ask_rerank
compose_digest_cluster_draft = _p.compose_digest_cluster_draft
rank_feed_suggestions = _p.rank_feed_suggestions
generate_briefing_navigator = _p.generate_briefing_navigator
compose_ai_navigator_brief = _p.compose_ai_navigator_brief
generate_item_navigator = _p.generate_item_navigator
generate_audio_briefing_script = _p.generate_audio_briefing_script
generate_ask_navigator = _p.generate_ask_navigator
generate_source_navigator = _p.generate_source_navigator
suggest_feed_seed_sites = _p.suggest_feed_seed_sites

extract_facts_async = _p.extract_facts_async
summarize_async = _p.summarize_async
check_summary_faithfulness_async = _p.check_summary_faithfulness_async
check_facts_async = _p.check_facts_async
translate_title_async = _p.translate_title_async
compose_digest_async = _p.compose_digest_async
ask_question_async = _p.ask_question_async
ask_rerank_async = _p.ask_rerank_async
compose_digest_cluster_draft_async = _p.compose_digest_cluster_draft_async
rank_feed_suggestions_async = _p.rank_feed_suggestions_async
generate_briefing_navigator_async = _p.generate_briefing_navigator_async
compose_ai_navigator_brief_async = _p.compose_ai_navigator_brief_async
generate_item_navigator_async = _p.generate_item_navigator_async
generate_audio_briefing_script_async = _p.generate_audio_briefing_script_async
generate_ask_navigator_async = _p.generate_ask_navigator_async
generate_source_navigator_async = _p.generate_source_navigator_async
suggest_feed_seed_sites_async = _p.suggest_feed_seed_sites_async
