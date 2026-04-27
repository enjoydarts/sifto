import os

from .provider_base import ProviderConfig, OpenAICompatProvider, env_timeout_seconds
from .llm_text_utils import (
    extract_first_json_object as _extract_first_json_object,
    extract_json_string_value_loose as _extract_json_string_value_loose,
    parse_json_string_array as _parse_json_string_array,
    strip_code_fence as _strip_code_fence,
)
from .title_translation_common import run_title_translation
from .openai_compat_transport import run_chat_json, run_chat_json_async
from .openai_responses_transport import run_responses_json, run_responses_json_async
from .llm_catalog import model_supports
from .task_transport_common import wrap_usage_transport


class _OpenAIProvider(OpenAICompatProvider):
    def _should_use_responses_api(self, model: str) -> bool:
        return self._normalize_model_family(model).startswith("gpt-5")

    def _supports_custom_temperature(self, model: str) -> bool:
        return not self._normalize_model_family(model).startswith("gpt-5")

    def _responses_reasoning(self, model: str) -> dict | None:
        family = self._normalize_model_family(model)
        if not family.startswith("gpt-5"):
            return None
        if family.endswith("-pro"):
            return None
        if family.startswith("gpt-5.1") or family.startswith("gpt-5.2") or family.startswith("gpt-5.4"):
            return {"effort": "none"}
        return {"effort": "minimal"}

    def _responses_json(self, prompt, model, api_key, **kwargs):
        return run_responses_json(
            prompt,
            model,
            api_key,
            normalize_model_name=self._normalize_model_name,
            responses_reasoning=self._responses_reasoning,
            supports_strict_schema=self._supports_strict_schema,
            **kwargs,
        )

    def _chat_json(self, prompt, model, api_key, **kwargs):
        if self._should_use_responses_api(model):
            return self._responses_json(prompt, model, api_key, **kwargs)
        return super()._chat_json(prompt, model, api_key, **kwargs)

    async def _chat_json_async(self, prompt, model, api_key, **kwargs):
        if self._should_use_responses_api(model):
            return await self._responses_json_async(prompt, model, api_key, **kwargs)
        return await super()._chat_json_async(prompt, model, api_key, **kwargs)

    async def _responses_json_async(self, prompt, model, api_key, **kwargs):
        return await run_responses_json_async(
            prompt,
            model,
            api_key,
            normalize_model_name=self._normalize_model_name,
            responses_reasoning=self._responses_reasoning,
            supports_strict_schema=self._supports_strict_schema,
            **kwargs,
        )

    def _translate_title_to_ja(self, title: str, model: str, api_key: str) -> str:
        src = (title or "").strip()
        system_instruction = """# Role
あなたは見出し翻訳の専門家です。

# Task
英語を含め、日本語以外のニュース記事タイトルを自然な日本語に翻訳してください。

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
タイトル: {src}
"""
        plain_prompt = f"""# Input
次のタイトルが日本語以外なら自然な日本語に翻訳してください。
説明・JSON・引用符は不要です。翻訳結果のみを1行で返してください。
原文をそのまま繰り返さず、日本語の文字を必ず含めてください。

タイトル: {src}
"""
        retry_prompt = f"""あなたはニュース見出し翻訳者です。
次のタイトルが日本語以外なら、日本のニュースアプリに載せる自然な日本語見出しへ翻訳してください。
出力は翻訳後タイトル1行のみです。説明、引用符、原文の反復は禁止です。

タイトル: {src}
"""
        return run_title_translation(
            src,
            structured_call=lambda: str(
                (_extract_first_json_object(
                    self._chat_json(
                        prompt,
                        model,
                        api_key,
                        system_instruction=system_instruction,
                        max_output_tokens=180,
                        response_schema={"type": "object", "properties": {"translated_title": {"type": "string"}}, "required": ["translated_title"]},
                        schema_name="translated_title",
                    )[0]
                ) or {}).get("translated_title")
                or ""
            ),
            plain_retry_call=lambda: self._chat_json(plain_prompt, model, api_key, max_output_tokens=120)[0],
            final_retry_call=lambda: self._chat_json(
                retry_prompt,
                model,
                api_key,
                system_instruction="出力は自然な日本語タイトル1行のみ。",
                max_output_tokens=120,
            )[0],
        )


_config = ProviderConfig(
    provider_name="openai",
    env_prefix="OPENAI",
    pricing_source_version="openai_standard_2026_03",
    api_base_url="https://api.openai.com/v1/chat/completions",
    api_base_url_env="OPENAI_API_BASE_URL",
    default_model="gpt-5",
    default_translate_model="gpt-5-mini",
    model_families=[
        "gpt-5.4-pro", "gpt-5.4", "gpt-5.2-pro", "gpt-5.2",
        "gpt-5.1", "gpt-5-pro", "gpt-5-mini", "gpt-5-nano", "gpt-5",
    ],
)
_p = _OpenAIProvider(_config)

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
