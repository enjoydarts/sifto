from .provider_base import ProviderConfig, OpenAICompatProvider

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
    "fireworks/qwen3p6-plus": {"input_per_mtok_usd": 0.50, "output_per_mtok_usd": 3.00, "cache_read_per_mtok_usd": 0.10},
}

_config = ProviderConfig(
    provider_name="fireworks",
    env_prefix="FIREWORKS",
    pricing_source_version="fireworks_static_2026_03",
    api_base_url="https://api.fireworks.ai/inference/v1/chat/completions",
    api_base_url_env="FIREWORKS_API_BASE_URL",
    legacy_model_pricing=_LEGACY_MODEL_PRICING,
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
