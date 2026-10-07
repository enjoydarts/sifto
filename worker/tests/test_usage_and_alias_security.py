from app.services.anthropic_transport import supports_sampling_parameters
from app.services.facts_task_common import parse_facts_result
from app.services.facts_check_runner import run_facts_check
from app.services.feed_task_common import parse_audio_briefing_script_result
import asyncio
from types import SimpleNamespace
from unittest.mock import AsyncMock, patch
from app.services import claude_service

def test_anthropic_latest_and_dated_aliases_omit_unsupported_sampling():
    for model in ("claude-opus-5-5", "claude-opus-5-5-latest", "claude-opus-5-5-20260901"):
        assert not supports_sampling_parameters(model)

def test_empty_structured_facts_do_not_become_field_names():
    assert parse_facts_result('{"facts":[]}') == []

def test_check_retry_accounts_for_both_provider_calls():
    result = run_facts_check(lambda: ('{"verdict":"pass"}', {"input_tokens": 10, "output_tokens": 3, "estimated_cost_usd": 0.1}), retry_call=lambda: ('{"verdict":"pass","short_comment":"本文で裏付けられています。"}', {"input_tokens": 12, "output_tokens": 4, "estimated_cost_usd": 0.2}))
    assert result["llm"]["input_tokens"] == 22
    assert result["llm"]["output_tokens"] == 7
    assert abs(result["llm"]["estimated_cost_usd"] - 0.3) < 0.00001

def test_audio_script_missing_headline_uses_verified_input_title():
    result = parse_audio_briefing_script_result('{"article_segments":[{"item_id":"1","summary_intro":"要約です。","commentary":"コメントです。"}]}', [{"item_id":"1","translated_title":"記事タイトル"}], "editor", include_opening=False, include_overall_summary=False, include_ending=False)
    assert result["article_segments"][0]["headline"] == "記事タイトル"

def test_claude_seed_rescue_accounts_for_both_calls_even_when_empty():
    def message(tokens):
        return SimpleNamespace(model="claude-sonnet-5-5", content=[SimpleNamespace(type="text", text='{"items":[]}')], usage=SimpleNamespace(input_tokens=tokens, output_tokens=2, cache_creation_input_tokens=0, cache_read_input_tokens=0))
    calls = AsyncMock(side_effect=[(message(10), "claude-sonnet-5-5", []), (message(12), "claude-sonnet-5-5", [])])
    with patch.object(claude_service, "_call_with_model_fallback_async", calls):
        result = asyncio.run(claude_service.suggest_feed_seed_sites_async([], [], api_key="test", model="claude-sonnet-5-5"))
    assert result["items"] == []
    assert calls.await_count == 2
    assert result["llm"]["input_tokens"] == 22
    assert result["llm"]["output_tokens"] == 4
    assert result["llm"]["estimated_cost_usd"] > 0
