import asyncio
import json
from types import SimpleNamespace

import anthropic
import httpx
import pytest

from app.services import anthropic_transport, claude_service
from app.services.llm_catalog import load_llm_catalog, model_pricing, provider_for_model


MODEL = "claude-haiku-5-5"


def test_haiku_5_5_is_selectable_for_all_chat_purposes():
    assert provider_for_model(MODEL) == "anthropic"
    entry = next((item for item in load_llm_catalog()["chat_models"] if item["id"] == MODEL), None)
    assert entry is not None
    assert set(entry["available_purposes"]) == {"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"}
    assert entry["capabilities"]["supports_reasoning"]
    pricing = model_pricing(MODEL)
    assert pricing["input_per_mtok_usd"] == 0.1
    assert pricing["output_per_mtok_usd"] == 0.5
    assert pricing["long_context"]["input_token_threshold"] == 100_000


@pytest.mark.parametrize("input_tokens,cache_write,cache_read,cost", [
    (100_000, 0, 0, 0.0105),
    (100_001, 0, 0, 0.0525005),
    (1, 0, 100_000, 0.0075005),
    (1, 100_000, 0, 0.0650005),
    (500, 200, 300, 0.000578),
])
def test_haiku_5_5_prices_entire_request_at_prompt_length_tier(monkeypatch, input_tokens, cache_write, cache_read, cost):
    for suffix in ("INPUT_PER_MTOK_USD", "OUTPUT_PER_MTOK_USD", "CACHE_WRITE_PER_MTOK_USD", "CACHE_READ_PER_MTOK_USD"):
        monkeypatch.delenv("ANTHROPIC_SUMMARY_" + suffix, raising=False)
    message = SimpleNamespace(model=MODEL, usage=SimpleNamespace(
        input_tokens=input_tokens, output_tokens=1000,
        cache_creation_input_tokens=cache_write, cache_read_input_tokens=cache_read,
    ))
    meta = claude_service._llm_meta(message, "summary", MODEL)
    assert meta["pricing_model_family"] == MODEL
    assert meta["pricing_source"] == "anthropic_docs_2026_10"
    assert meta["estimated_cost_usd"] == cost


@pytest.mark.parametrize("async_call", [False, True], ids=["sync", "async"])
def test_haiku_5_5_facts_request_and_sdk_json(monkeypatch, async_call):
    requests = []

    def respond(request):
        body = json.loads(request.content)
        requests.append(body)
        assert body["model"] == MODEL
        assert body.get("thinking") == {"type": "disabled"}
        assert body["max_tokens"] == 1332
        assert "temperature" not in body
        assert "top_p" not in body
        return httpx.Response(200, json={
            "id": "msg_haiku", "type": "message", "role": "assistant", "model": MODEL,
            "content": [{"type": "text", "text": '["新しいモデルが公開された。"]'}],
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
    assert result["llm"]["estimated_cost_usd"] == 0.000578


@pytest.mark.parametrize("async_call", [False, True])
def test_haiku_5_5_omits_rejected_sampling_parameters(monkeypatch, async_call):
    def respond(request):
        body = json.loads(request.content)
        assert "temperature" not in body
        assert "top_p" not in body
        assert body.get("thinking") == {"type": "disabled"}
        assert body["max_tokens"] == 260
        return httpx.Response(200, json={
            "id": "msg_haiku", "type": "message", "role": "assistant", "model": MODEL,
            "content": [{"type": "text", "text": "{}"}], "stop_reason": "end_turn", "stop_sequence": None,
            "usage": {"input_tokens": 10, "output_tokens": 2},
        })
    kwargs = dict(prompt="prompt", model=MODEL, max_tokens=200, api_key="test-key", temperature=0.2, top_p=0.8)
    if async_call:
        async def run():
            async with anthropic.AsyncAnthropic(api_key="test-key", http_client=httpx.AsyncClient(transport=httpx.MockTransport(respond))) as client:
                monkeypatch.setattr(anthropic_transport, "async_client_for_api_key", lambda *args, **kwargs: client)
                return await anthropic_transport.messages_create_async(**kwargs)
        message = asyncio.run(run())
    else:
        with anthropic.Anthropic(api_key="test-key", http_client=httpx.Client(transport=httpx.MockTransport(respond))) as client:
            monkeypatch.setattr(anthropic_transport, "client_for_api_key", lambda *args, **kwargs: client)
            message = anthropic_transport.messages_create(**kwargs)
    assert anthropic_transport.message_text(message) == "{}"


@pytest.mark.parametrize("async_call", [False, True])
def test_haiku_5_5_rejects_response_without_answer_text(monkeypatch, async_call):
    def respond(request):
        return httpx.Response(200, json={
            "id": "msg_haiku", "type": "message", "role": "assistant", "model": MODEL,
            "content": [{"type": "thinking", "thinking": "", "signature": "test-signature"}],
            "stop_reason": "max_tokens", "stop_sequence": None,
            "usage": {"input_tokens": 10, "output_tokens": 200},
        })
    kwargs = dict(prompt="prompt", model=MODEL, max_tokens=200, api_key="test-key")
    with pytest.raises(RuntimeError, match="returned no text"):
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
