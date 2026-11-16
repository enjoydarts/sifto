import asyncio
import json
from datetime import datetime, timezone
from unittest.mock import Mock

import httpx
import pytest

from app.services import gemini_transport, gemini_service
from app.services.llm_catalog import load_llm_catalog, model_pricing, provider_for_model

MODELS = ("gemini-4-carbon", "gemini-4-argon", "gemini-4-hydrogen")
PURPOSES = {"facts", "summary", "digest_cluster_draft", "digest", "ask", "source_suggestion"}


@pytest.mark.parametrize("model", MODELS)
def test_gemini_4_catalog(model):
    entry = next((m for m in load_llm_catalog()["chat_models"] if m["id"] == model), None)
    assert entry is not None
    assert provider_for_model(model) == "google"
    assert set(entry["available_purposes"]) == PURPOSES
    assert "latest" in entry["highlights"]
    assert entry["capabilities"]["supports_structured_output"]


@pytest.mark.parametrize("model", MODELS)
@pytest.mark.parametrize("at, expected", [
    (datetime(2026, 11, 30, 23, 59, 59, tzinfo=timezone.utc), (2, 10, 0.1)),
    (datetime(2026, 12, 1, tzinfo=timezone.utc), (4, 20, 0.2)),
])
def test_gemini_4_pricing_cutover(model, at, expected):
    if model.endswith("hydrogen"):
        expected = (1.5, 7.5, 0.075)
    pricing = model_pricing(model, at=at)
    assert tuple(pricing[k] for k in ("input_per_mtok_usd", "output_per_mtok_usd", "cache_read_per_mtok_usd")) == expected


def call_transport(monkeypatch, model, async_call, respond, **kwargs):
    client_class = httpx.Client
    async_class = httpx.AsyncClient
    transport = httpx.MockTransport(respond)
    monkeypatch.setattr(gemini_transport.httpx, "Client", lambda **kw: client_class(transport=transport, **kw))
    monkeypatch.setattr(gemini_transport.httpx, "AsyncClient", lambda **kw: async_class(transport=transport, **kw))
    options = dict(normalize_model_name=gemini_service._normalize_model_name, logger=Mock(), **kwargs)
    if async_call:
        return asyncio.run(gemini_transport.generate_content_async("prompt", model, "test-key", **options))
    return gemini_transport.generate_content("prompt", model, "test-key", **options)


@pytest.mark.parametrize("model", MODELS)
@pytest.mark.parametrize("async_call", [False, True])
def test_gemini_4_native_json_without_explicit_cache(monkeypatch, model, async_call):
    requests = []

    def respond(request):
        assert request.url.path.endswith(":generateContent"), "explicit caching is unsupported"
        body = json.loads(request.content)
        requests.append(body)
        config = body["generationConfig"]
        assert not {"temperature", "topP", "topK"} & config.keys()
        assert config["maxOutputTokens"] == (65536 if model.endswith("hydrogen") else 32768)
        assert "cachedContent" not in body
        assert body["systemInstruction"]["parts"][0]["text"] == "system"
        assert config["responseSchema"]["type"] == "ARRAY"
        return httpx.Response(200, json={
            "candidates": [{"finishReason": "STOP", "content": {"parts": [
                {"thought": True, "text": "private reasoning"}, {"text": '["answer"]'},
            ]}}],
            "usageMetadata": {"promptTokenCount": 1000, "cachedContentTokenCount": 200,
                              "candidatesTokenCount": 100, "thoughtsTokenCount": 500},
        })

    text, usage = call_transport(monkeypatch, "models/" + model, async_call, respond,
        max_output_tokens=120, temperature=0.2, top_p=0.8,
        system_instruction="system", context_cache_key="gemini4-cache-test",
        response_schema={"type": "ARRAY", "items": {"type": "STRING"}})
    assert len(requests) == 1
    assert text == '["answer"]'
    assert usage["output_tokens"] == 600
    assert usage["input_tokens"] == 1000
    assert usage["cache_read_input_tokens"] == 200


@pytest.mark.parametrize("async_call", [False, True])
@pytest.mark.parametrize("reason, parts", [
    ("STOP", []), ("STOP", [{"thought": True, "text": "thinking"}]),
    ("MAX_TOKENS", [{"text": '["partial"'}]), ("SAFETY", []),
])
def test_gemini_4_rejects_empty_or_incomplete_answer(monkeypatch, async_call, reason, parts):
    def respond(_request):
        return httpx.Response(200, json={"candidates": [{"finishReason": reason, "content": {"parts": parts}}]})
    with pytest.raises(RuntimeError, match="Gemini 4"):
        call_transport(monkeypatch, "gemini-4-hydrogen", async_call, respond)
