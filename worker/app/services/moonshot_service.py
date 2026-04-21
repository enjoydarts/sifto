from .provider_base import ProviderConfig, OpenAICompatProvider


class _MoonshotProvider(OpenAICompatProvider):
    def _normalize_temperature(self, model: str, value: float | None) -> float:
        m = self._normalize_model_name(model)
        if m in {"kimi-k2.5", "kimi-k2.6"}:
            return 0.6
        return 1.0

    def _normalize_top_p(self, model: str, value: float | None) -> float:
        return 0.95


_config = ProviderConfig(
    provider_name="moonshot",
    env_prefix="MOONSHOT",
    pricing_source_version="moonshot_static_2026_03",
    api_base_url="https://api.moonshot.ai/v1/chat/completions",
    api_base_url_env="MOONSHOT_API_BASE_URL",
    use_billed_cost=True,
)
_p = _MoonshotProvider(_config)
_llm_meta = _p._llm_meta
_normalize_temperature = _p._normalize_temperature
_normalize_top_p = _p._normalize_top_p

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
