import asyncio
import json

import anthropic
import httpx2 as httpx
import pytest

from app.services import anthropic_transport, claude_service
from app.services.llm_catalog import load_llm_catalog, model_pricing, provider_for_model


MODEL = "claude-fable-5-2"


def test_fable_5_2_catalog_and_standard_cache_prices():
    entry = next((item for item in load_llm_catalog()["chat_models"] if item["id"] == MODEL), None)
    assert entry is not None
    assert provider_for_model(MODEL) == "anthropic"
    assert set(entry["available_purposes"]) == {"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"}
    assert entry["capabilities"]["supports_structured_output"]
    assert entry["capabilities"]["supports_reasoning"]
    pricing = model_pricing(MODEL)
    assert pricing["input_per_mtok_usd"] == 8
    assert pricing["output_per_mtok_usd"] == 40
    assert pricing["cache_write_per_mtok_usd"] == 10
    assert pricing["cache_read_per_mtok_usd"] == 0.2


@pytest.mark.parametrize("async_call", [False, True], ids=["sync", "async"])
def test_fable_5_2_facts_sdk_request_and_usage(monkeypatch, async_call):
    for suffix in ("INPUT_PER_MTOK_USD", "OUTPUT_PER_MTOK_USD", "CACHE_WRITE_PER_MTOK_USD", "CACHE_READ_PER_MTOK_USD"):
        monkeypatch.delenv("ANTHROPIC_FACTS_" + suffix, raising=False)
    requests = []

    def respond(request):
        body = json.loads(request.content)
        requests.append(body)
        assert body["model"] == MODEL
        assert body.get("thinking", {"type": "adaptive"}) == {"type": "adaptive"}
        assert not {"temperature", "top_p", "top_k", "tool_choice"} & body.keys()
        return httpx.Response(200, json={
            "id": "msg_fable", "type": "message", "role": "assistant", "model": MODEL,
            "content": [
                {"type": "thinking", "thinking": "Checking the facts", "signature": "test-signature"},
                {"type": "text", "text": '["新しいモデルが公開された。"]'},
            ],
            "stop_reason": "end_turn", "stop_sequence": None,
            "usage": {"input_tokens": 500, "output_tokens": 1000, "cache_creation_input_tokens": 200, "cache_read_input_tokens": 300},
        })

    kwargs = dict(title="モデル公開", content="新しいモデルが公開された。", api_key="test-key", model=MODEL)
    if async_call:
        async def run():
            async with anthropic.AsyncAnthropic(api_key="test-key", http_client=httpx.AsyncClient(transport=httpx.MockTransport(respond))) as client:
                monkeypatch.setattr(anthropic_transport, "async_client_for_api_key", lambda *args, **kwargs: client)
                return await claude_service.extract_facts_async(**kwargs)
        result = asyncio.run(run())
    else:
        with anthropic.Anthropic(api_key="test-key", http_client=httpx.Client(transport=httpx.MockTransport(respond))) as client:
            monkeypatch.setattr(anthropic_transport, "client_for_api_key", lambda *args, **kwargs: client)
            result = claude_service.extract_facts(**kwargs)
    assert len(requests) == 1
    assert result["facts"] == ["新しいモデルが公開された。"]
    assert result["llm"]["pricing_model_family"] == MODEL
    assert result["llm"]["pricing_source"] == "anthropic_docs_2026_11"
    assert result["llm"]["estimated_cost_usd"] == 0.04606


@pytest.mark.parametrize("async_call", [False, True], ids=["sync", "async"])
@pytest.mark.parametrize("stop_reason", ["end_turn", "max_tokens", "refusal"])
def test_fable_5_2_rejects_empty_answer_and_omits_sampling(monkeypatch, async_call, stop_reason):
    def respond(request):
        body = json.loads(request.content)
        assert not {"temperature", "top_p", "top_k", "tool_choice"} & body.keys()
        assert body.get("thinking", {"type": "adaptive"}) == {"type": "adaptive"}
        return httpx.Response(200, json={
            "id": "msg_empty", "type": "message", "role": "assistant", "model": MODEL,
            "content": [{"type": "thinking", "thinking": "", "signature": "test-signature"}],
            "stop_reason": stop_reason, "stop_sequence": None,
            "usage": {"input_tokens": 10, "output_tokens": 200},
        })

    kwargs = dict(prompt="prompt", model=MODEL, api_key="test-key", temperature=0.2, top_p=0.8)
    with pytest.raises(RuntimeError, match=f"returned no text.*stop_reason={stop_reason}"):
        if async_call:
            async def run():
                async with anthropic.AsyncAnthropic(api_key="test-key", http_client=httpx.AsyncClient(transport=httpx.MockTransport(respond))) as client:
                    monkeypatch.setattr(anthropic_transport, "async_client_for_api_key", lambda *args, **kwargs: client)
                    return await anthropic_transport.messages_create_async(**kwargs)
            asyncio.run(run())
        else:
            with anthropic.Anthropic(api_key="test-key", http_client=httpx.Client(transport=httpx.MockTransport(respond))) as client:
                monkeypatch.setattr(anthropic_transport, "client_for_api_key", lambda *args, **kwargs: client)
                anthropic_transport.messages_create(**kwargs)
