import re

import httpx

from .provider_base import ProviderConfig, OpenAICompatProvider
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


class _SiliconFlowProvider(OpenAICompatProvider):
    def _normalize_model_family(self, model: str) -> str:
        return _canonicalize_resolved_model(model, self)

    def _fetch_generation_cost_details(self, api_key: str, generation_id: str, timeout_sec: float) -> dict:
        return {}

    def _post_process_chat_result(self, text, usage, model, api_key, response_schema, timeout):
        if usage.get("billed_cost_usd") is None and usage.get("generation_id"):
            generation_meta = self._fetch_generation_cost_details(api_key, str(usage.get("generation_id") or ""), timeout)
            if generation_meta.get("billed_cost_usd") is not None:
                usage["billed_cost_usd"] = generation_meta["billed_cost_usd"]
            if generation_meta.get("resolved_model") and not usage.get("resolved_model"):
                usage["resolved_model"] = generation_meta["resolved_model"]
        return _repair_structured_json_text(text, model, response_schema), usage


_config = ProviderConfig(
    provider_name="siliconflow",
    env_prefix="SILICONFLOW",
    pricing_source_version="siliconflow_snapshot",
    api_base_url="https://api.siliconflow.com/v1/chat/completions",
    api_base_url_env="SILICONFLOW_API_BASE_URL",
    use_resolve_model_id=True,
    use_billed_cost=True,
    include_resolved_model_in_meta=True,
)
_p = _SiliconFlowProvider(_config)

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
