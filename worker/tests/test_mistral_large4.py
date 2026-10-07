import asyncio
import json

import httpx
import pytest

from app.services import mistral_service, openai_compat_transport
from app.services.llm_catalog import load_llm_catalog, model_pricing, provider_for_model


def test_mistral_large4_is_selectable_with_standard_pricing():
    model = "mistral-large-4"
    assert provider_for_model(model) == "mistral"
    assert provider_for_model("mistral-large-4-0") == "mistral"
    entry = next(item for item in load_llm_catalog()["chat_models"] if item["id"] == model)
    assert set(entry["available_purposes"]) == {"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"}
    pricing = model_pricing(model)
    assert pricing["input_per_mtok_usd"] == 1.36
    assert pricing["output_per_mtok_usd"] == 4.18
    assert pricing["cache_read_per_mtok_usd"] == 0.14


@pytest.mark.parametrize("async_call", [False, True], ids=["sync", "async"])
@pytest.mark.parametrize("model", ["mistral-large-4", "mistral-large-4-0"])
def test_mistral_large4_extracts_json_facts_and_accounts_for_cached_input(monkeypatch, async_call, model):
    for suffix in ("INPUT_PER_MTOK_USD", "OUTPUT_PER_MTOK_USD", "CACHE_READ_PER_MTOK_USD"):
        monkeypatch.delenv(f"MISTRAL_FACTS_{suffix}", raising=False)
    monkeypatch.delenv("MISTRAL_API_BASE_URL", raising=False)
    monkeypatch.setenv("MISTRAL_RETRY_ATTEMPTS", "1")
    requests = []

    def respond(request):
        requests.append(request)
        body = json.loads(request.content)
        assert str(request.url) == "https://api.mistral.ai/v1/chat/completions"
        assert request.headers["Authorization"] == "Bearer test-key"
        assert body["model"] == model
        assert body["response_format"] == {"type": "json_object"}
        assert body.get("reasoning_effort") == "none"
        return httpx.Response(200, json={
            "model": model,
            "choices": [{"message": {"content": '{"facts":["新しいモデルが公開された。"]}'}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 1000, "completion_tokens": 200, "prompt_tokens_details": {"cached_tokens": 400}},
        })

    transport = httpx.MockTransport(respond)
    client_class = httpx.AsyncClient if async_call else httpx.Client
    monkeypatch.setattr(
        openai_compat_transport.httpx,
        "AsyncClient" if async_call else "Client",
        lambda **kwargs: client_class(transport=transport, **kwargs),
    )
    kwargs = {"title": "モデル公開", "content": "新しいモデルが公開された。", "model": model, "api_key": "test-key"}
    if async_call:
        result = asyncio.run(mistral_service.extract_facts_async(**kwargs))
    else:
        result = mistral_service.extract_facts(**kwargs)

    assert len(requests) == 1
    assert result["facts"] == ["新しいモデルが公開された。"]
    assert result["llm"]["provider"] == "mistral"
    assert result["llm"]["model"] == model
    assert result["llm"]["pricing_model_family"] == "mistral-large-4"
    assert result["llm"]["cache_read_input_tokens"] == 400
    assert result["llm"]["pricing_source"] == "mistral_docs_2026_10_standard"
    assert result["llm"]["estimated_cost_usd"] == 0.001708
