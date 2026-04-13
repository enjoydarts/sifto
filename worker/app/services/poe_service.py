import os
import time

import httpx

from .provider_base import ProviderConfig, OpenAICompatProvider
from .llm_text_utils import extract_first_json_object as _extract_first_json_object
from .openai_compat_transport import run_chat_json
from .task_transport_common import with_execution_failures
from .llm_catalog import model_pricing


def _env_optional_float(name: str) -> float | None:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return None
    try:
        return float(raw)
    except Exception:
        return None


def _load_repair_json():
    try:
        from json_repair import repair_json
    except Exception:
        return None
    return repair_json


def _repair_structured_json_text(text: str, model: str, response_schema) -> str:
    cleaned = str(text or "").strip()
    if not cleaned or response_schema is None:
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


class _PoeProvider(OpenAICompatProvider):
    def _normalize_model_family(self, model: str) -> str:
        return self._normalize_model_name(model)

    def _pricing_for_model(self, model: str, purpose: str) -> dict:
        family = self._normalize_model_family(model)
        base = dict(model_pricing(model) or model_pricing(family) or {"input_per_mtok_usd": 0.0, "output_per_mtok_usd": 0.0, "cache_read_per_mtok_usd": 0.0})
        source = str(base.get("pricing_source") or self.config.pricing_source_version)
        prefix = f"{self.config.env_prefix}_{purpose.upper()}_"
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
        base["pricing_model_family"] = self._normalize_model_name(model)
        return base

    def _supports_anthropic_transport(self, model: str) -> bool:
        resolved = self._normalize_model_name(model).lower()
        return resolved.startswith("claude") or resolved.startswith("anthropic/claude")

    def _anthropic_usage_from_response(self, data: dict) -> dict:
        usage = data.get("usage") or {}
        return {
            "input_tokens": int(usage.get("input_tokens", 0) or 0),
            "output_tokens": int(usage.get("output_tokens", 0) or 0),
            "cache_creation_input_tokens": int(usage.get("cache_creation_input_tokens", 0) or 0),
            "cache_read_input_tokens": int(usage.get("cache_read_input_tokens", 0) or 0),
            "resolved_model": str(data.get("model") or "").strip() or None,
        }

    def _chat_json_openai_compat(self, prompt, model, api_key, **kwargs):
        from .provider_base import env_timeout_seconds
        req_timeout = kwargs.get("timeout_sec") if kwargs.get("timeout_sec") and kwargs.get("timeout_sec") > 0 else env_timeout_seconds(f"{self.config.env_prefix}_TIMEOUT_SEC", 300.0)
        attempts = max(1, int(os.getenv(f"{self.config.env_prefix}_RETRY_ATTEMPTS", "3") or "3"))
        base_sleep_sec = env_timeout_seconds(f"{self.config.env_prefix}_RETRY_BASE_SEC", 0.5)
        text, usage = run_chat_json(
            prompt,
            model,
            api_key,
            url=os.getenv("POE_OPENAI_API_BASE_URL", "https://api.poe.com/v1/chat/completions"),
            normalize_model_name=self._normalize_model_name,
            supports_strict_schema=self._supports_strict_schema,
            timeout_sec=req_timeout,
            attempts=attempts,
            base_sleep_sec=base_sleep_sec,
            provider_name=self.config.provider_name,
            logger=self._log,
            system_instruction=kwargs.get("system_instruction"),
            max_output_tokens=kwargs.get("max_output_tokens", 1200),
            response_schema=kwargs.get("response_schema"),
            schema_name=kwargs.get("schema_name", "response"),
            include_temperature=True,
            temperature=kwargs.get("temperature"),
            top_p=kwargs.get("top_p"),
        )
        return _repair_structured_json_text(text, model, kwargs.get("response_schema")), usage

    def _chat_json_anthropic_compat(self, prompt, model, api_key, **kwargs):
        from .provider_base import env_timeout_seconds
        req_timeout = kwargs.get("timeout_sec") if kwargs.get("timeout_sec") and kwargs.get("timeout_sec") > 0 else env_timeout_seconds(f"{self.config.env_prefix}_TIMEOUT_SEC", 300.0)
        attempts = max(1, int(os.getenv(f"{self.config.env_prefix}_RETRY_ATTEMPTS", "3") or "3"))
        base_sleep_sec = env_timeout_seconds(f"{self.config.env_prefix}_RETRY_BASE_SEC", 0.5)
        body: dict = {
            "model": self._normalize_model_name(model),
            "max_tokens": kwargs.get("max_output_tokens", 1200),
            "messages": [{"role": "user", "content": prompt}],
        }
        if kwargs.get("system_instruction"):
            body["system"] = kwargs["system_instruction"]
        if kwargs.get("temperature") is not None:
            body["temperature"] = kwargs["temperature"]
        if kwargs.get("top_p") is not None:
            body["top_p"] = kwargs["top_p"]
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
        usage = self._anthropic_usage_from_response(data)
        usage["requested_model"] = requested_model
        if retry_usage.get("execution_failures"):
            usage["execution_failures"] = list(retry_usage["execution_failures"])
        return _repair_structured_json_text(text, model, kwargs.get("response_schema")), usage

    def _chat_json(self, prompt, model, api_key, **kwargs):
        api_key = (api_key or "").strip()
        if not api_key:
            raise RuntimeError("poe api key is required")
        if self._supports_anthropic_transport(model):
            return self._chat_json_anthropic_compat(prompt, model, api_key, **kwargs)
        return self._chat_json_openai_compat(prompt, model, api_key, **kwargs)


_config = ProviderConfig(
    provider_name="poe",
    env_prefix="POE",
    pricing_source_version="poe_snapshot",
    api_base_url="https://api.poe.com/v1/chat/completions",
    api_base_url_env="POE_OPENAI_API_BASE_URL",
    use_resolve_model_id=True,
    use_billed_cost=True,
    include_resolved_model_in_meta=True,
)
_p = _PoeProvider(_config)
_chat_json = _p._chat_json
_llm_meta = _p._llm_meta
_supports_anthropic_transport = _p._supports_anthropic_transport

extract_facts = _p.extract_facts
summarize = _p.summarize
check_summary_faithfulness = _p.check_summary_faithfulness
check_facts = _p.check_facts
translate_title = _p.translate_title
compose_digest = _p.compose_digest
ask_question = _p.ask_question
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
compose_digest_cluster_draft_async = _p.compose_digest_cluster_draft_async
rank_feed_suggestions_async = _p.rank_feed_suggestions_async
generate_briefing_navigator_async = _p.generate_briefing_navigator_async
compose_ai_navigator_brief_async = _p.compose_ai_navigator_brief_async
generate_item_navigator_async = _p.generate_item_navigator_async
generate_audio_briefing_script_async = _p.generate_audio_briefing_script_async
generate_ask_navigator_async = _p.generate_ask_navigator_async
generate_source_navigator_async = _p.generate_source_navigator_async
suggest_feed_seed_sites_async = _p.suggest_feed_seed_sites_async
