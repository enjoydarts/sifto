import os
import re
from contextlib import contextmanager
from contextvars import ContextVar

from .provider_base import ProviderConfig, OpenAICompatProvider


_REQUEST_WORKSPACE = ContextVar("alibaba_request_workspace", default=None)


@contextmanager
def alibaba_workspace_context(workspace_id: str):
    token = _REQUEST_WORKSPACE.set(str(workspace_id or "").strip())
    try:
        yield
    finally:
        _REQUEST_WORKSPACE.reset(token)


class AlibabaProvider(OpenAICompatProvider):
    def _get_chat_url(self) -> str:
        workspace = _REQUEST_WORKSPACE.get()
        if workspace is not None:
            if not re.fullmatch(r"ws-[a-z0-9]{1,60}", workspace):
                raise RuntimeError("valid Alibaba workspace ID is required for the Tokyo endpoint")
            return f"https://{workspace}.ap-northeast-1.maas.aliyuncs.com/compatible-mode/v1/chat/completions"
        base = os.getenv(self.config.api_base_url_env, "").strip().rstrip("/")
        if not base:
            raise RuntimeError("ALIBABA_API_BASE_URL is required; configure the Tokyo workspace endpoint")
        if base.endswith("/chat/completions"):
            return base
        return base + "/chat/completions"


_config = ProviderConfig(
    provider_name="alibaba",
    env_prefix="ALIBABA",
    pricing_source_version="alibaba_modelstudio_tokyo_global_2026_10_10",
    api_base_url="",
    api_base_url_env="ALIBABA_API_BASE_URL",
)
_p = AlibabaProvider(_config)

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
