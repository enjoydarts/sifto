import asyncio
from types import SimpleNamespace
from unittest.mock import Mock

import pytest

from app.services import anthropic_transport
from app.services.llm_catalog import model_pricing, provider_for_model


@pytest.mark.parametrize("model,rates", [
    ("claude-fable-5-2", (8, 40, 10, 0.2)),
    ("claude-opus-5-5", (4, 20, 5, 0.2)),
    ("claude-sonnet-5-5", (2, 10, 2.5, 0.1)),
    ("claude-haiku-5-5", (0.1, 0.5, 0.125, 0.01)),
])
def test_latest_anthropic_standard_prices(model, rates):
    assert provider_for_model(model) == "anthropic"
    pricing = model_pricing(model)
    assert tuple(pricing[k] for k in ("input_per_mtok_usd", "output_per_mtok_usd", "cache_write_per_mtok_usd", "cache_read_per_mtok_usd")) == rates


@pytest.mark.parametrize("model", ["claude-fable-5-2", "claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"])
@pytest.mark.parametrize("async_call", [False, True])
def test_latest_anthropic_empty_text_is_an_error(monkeypatch, model, async_call):
    response = SimpleNamespace(content=[SimpleNamespace(type="thinking", thinking="reasoning")], stop_reason="max_tokens")
    if async_call:
        class Messages:
            async def create(self, **_kwargs): return response
        monkeypatch.setattr(anthropic_transport, "async_client_for_api_key", lambda *a, **kw: SimpleNamespace(messages=Messages()))
    else:
        monkeypatch.setattr(anthropic_transport, "client_for_api_key", lambda *a, **kw: SimpleNamespace(messages=SimpleNamespace(create=Mock(return_value=response))))
    with pytest.raises(RuntimeError, match="returned no text"):
        if async_call:
            asyncio.run(anthropic_transport.messages_create_async("prompt", model, api_key="test-key"))
        else:
            anthropic_transport.messages_create("prompt", model, api_key="test-key")
