from .provider_base import ProviderConfig, OpenAICompatProvider

_LEGACY_MODEL_PRICING = {
    "openai/gpt-oss-20b": {"input_per_mtok_usd": 0.075, "output_per_mtok_usd": 0.30, "cache_read_per_mtok_usd": 0.0375},
    "openai/gpt-oss-120b": {"input_per_mtok_usd": 0.15, "output_per_mtok_usd": 0.60, "cache_read_per_mtok_usd": 0.075},
    "llama-3.1-8b-instant": {"input_per_mtok_usd": 0.05, "output_per_mtok_usd": 0.08, "cache_read_per_mtok_usd": 0.0},
    "llama-3.3-70b-versatile": {"input_per_mtok_usd": 0.59, "output_per_mtok_usd": 0.79, "cache_read_per_mtok_usd": 0.0},
    "meta-llama/llama-4-scout-17b-16e-instruct": {"input_per_mtok_usd": 0.11, "output_per_mtok_usd": 0.34, "cache_read_per_mtok_usd": 0.0},
    "qwen/qwen3-32b": {"input_per_mtok_usd": 0.29, "output_per_mtok_usd": 0.59, "cache_read_per_mtok_usd": 0.0},
    "moonshotai/kimi-k2-instruct-0905": {"input_per_mtok_usd": 1.0, "output_per_mtok_usd": 3.0, "cache_read_per_mtok_usd": 0.5},
}

_config = ProviderConfig(
    provider_name="groq",
    env_prefix="GROQ",
    pricing_source_version="groq_static_2026_03",
    api_base_url="https://api.groq.com/openai/v1/chat/completions",
    api_base_url_env="GROQ_API_BASE_URL",
    default_model="openai/gpt-oss-120b",
    default_translate_model="openai/gpt-oss-20b",
    legacy_model_pricing=_LEGACY_MODEL_PRICING,
    facts_output_mode="array",
    facts_pass_schema=False,
)
_p = OpenAICompatProvider(_config)

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
