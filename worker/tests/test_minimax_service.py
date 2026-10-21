import asyncio
import json
import os
import unittest
from unittest.mock import patch

import httpx

from app.services.llm_catalog import provider_api_key_header, provider_for_model
from app.services.minimax_service import (
    _chat_completions_url,
    _p,
    _llm_meta,
    _require_model,
    summarize,
    summarize_async,
)


class MiniMaxServiceTests(unittest.TestCase):
    def _check_flash_request(self, request):
        body = json.loads(request.content)
        self.assertEqual(request.headers["Authorization"], "Bearer minimax-key")
        self.assertEqual(body["model"], "MiniMax-M3.1-Flash")
        self.assertEqual(body.get("reasoning_effort"), "low")
        self.assertEqual(body.get("max_completion_tokens"), 4096)
        self.assertNotIn("max_tokens", body)
        self.assertNotIn("temperature", body)
        self.assertNotIn("top_p", body)
        self.assertNotEqual(body.get("thinking"), {"type": "disabled"})
        self.assertEqual(body["response_format"], {"type": "json_object"})
        return httpx.Response(200, json={
            "choices": [{"message": {"content": '{"answer":"ok"}', "reasoning_content": "reasoning"}}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 20},
        })

    def test_flash_requests_use_supported_reasoning_and_preserve_final_json(self):
        client = httpx.Client
        transport = httpx.MockTransport(self._check_flash_request)
        for model in ("MiniMax-M3.1-Flash", "minimax::MiniMax-M3.1-Flash", "minimax/MiniMax-M3.1-Flash"):
            with self.subTest(model=model), patch("app.services.openai_compat_transport.httpx.Client",
                    side_effect=lambda **kwargs: client(transport=transport, **kwargs)):
                text, usage = _p._chat_json("Return JSON", model, "minimax-key",
                    max_output_tokens=1200, response_schema={"type": "object"}, top_p=0.5)
                self.assertEqual(json.loads(text), {"answer": "ok"})
                self.assertEqual(usage["input_tokens"], 10)

    def test_flash_async_requests_use_same_supported_parameters(self):
        client = httpx.AsyncClient
        transport = httpx.MockTransport(self._check_flash_request)
        with patch("app.services.openai_compat_transport.httpx.AsyncClient",
                side_effect=lambda **kwargs: client(transport=transport, **kwargs)):
            text, usage = asyncio.run(_p._chat_json_async("Return JSON", "MiniMax-M3.1-Flash", "minimax-key",
                max_output_tokens=1200, response_schema={"type": "object"}, top_p=0.5))
        self.assertEqual(json.loads(text), {"answer": "ok"})
        self.assertEqual(usage["output_tokens"], 20)

    def test_flash_pricing_preserves_provider_and_cache_cost(self):
        llm = _llm_meta("minimax::MiniMax-M3.1-Flash", "summary",
            {"input_tokens": 1_000_000, "output_tokens": 100_000, "cache_read_input_tokens": 100_000})
        self.assertEqual(llm["provider"], "minimax")
        self.assertAlmostEqual(llm["estimated_cost_usd"], 0.1965)

    def test_flash_non_summary_truncation_grows_budget_sync_and_async(self):
        for asynchronous in (False, True):
            budgets = []

            def respond(request):
                body = json.loads(request.content)
                budgets.append(body["max_completion_tokens"])
                truncated = len(budgets) == 1
                return httpx.Response(200, json={
                    "choices": [{"finish_reason": "length" if truncated else "stop", "message": {
                        "content": "" if truncated else '{"answer":"ok"}', "reasoning_content": "thinking"}}],
                    "usage": {"prompt_tokens": 10, "completion_tokens": 100},
                })

            transport = httpx.MockTransport(respond)
            client_type = "AsyncClient" if asynchronous else "Client"
            client = getattr(httpx, client_type)
            with self.subTest(asynchronous=asynchronous), patch(
                "app.services.openai_compat_transport.httpx." + client_type,
                side_effect=lambda **kwargs: client(transport=transport, **kwargs),
            ):
                kwargs = {"max_output_tokens": 120, "schema_name": "verification", "response_schema": {"type": "object"}}
                if asynchronous:
                    text, usage = asyncio.run(_p._chat_json_async("Return JSON", "MiniMax-M3.1-Flash", "key", **kwargs))
                else:
                    text, usage = _p._chat_json("Return JSON", "MiniMax-M3.1-Flash", "key", **kwargs)
                self.assertEqual(json.loads(text), {"answer": "ok"})
                self.assertEqual(budgets, [4096, 8192])
                self.assertEqual(usage["output_tokens"], 200)

    def test_provider_for_model_detects_minimax(self):
        self.assertEqual(provider_for_model("MiniMax-M3"), "minimax")
        self.assertEqual(provider_for_model("MiniMax-M2.5"), "minimax")
        self.assertEqual(provider_for_model("MiniMax-M2.7"), "minimax")
        self.assertEqual(provider_for_model("minimax::MiniMax-M2.7"), "minimax")
        self.assertEqual(provider_for_model("minimax/MiniMax-M2.7"), "minimax")

    def test_provider_api_key_header_uses_minimax_header(self):
        self.assertEqual(provider_api_key_header("minimax"), "x-minimax-api-key")

    def test_model_is_required(self):
        with self.assertRaisesRegex(RuntimeError, "minimax model is required for summary"):
            _require_model(None, "summary")

    def test_chat_completions_url_defaults_to_openai_compatible_endpoint(self):
        self.assertEqual(_chat_completions_url(""), "https://api.minimax.io/v1/chat/completions")

    def test_chat_completions_url_normalizes_env_override(self):
        self.assertEqual(_chat_completions_url("https://api.minimax.io"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/v1"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/v1/chat/completions"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/anthropic"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/anthropic/v1"), "https://api.minimax.io/v1/chat/completions")

    @patch("app.services.provider_base.run_chat_json", return_value=("{}", {"input_tokens": 1, "output_tokens": 2}))
    def test_chat_json_uses_minimax_openai_compatible_url(self, chat_json):
        with patch.dict(os.environ, {"MINIMAX_API_BASE_URL": "https://api.minimax.io"}, clear=False):
            _p._chat_json("prompt", "MiniMax-M2.7", "minimax-key", max_output_tokens=128)
        self.assertEqual(chat_json.call_args.kwargs.get("url"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(chat_json.call_args.kwargs.get("provider_name"), "minimax")
        self.assertEqual(chat_json.call_args.kwargs.get("auth_header_name"), "Authorization")
        self.assertEqual(chat_json.call_args.kwargs.get("auth_scheme"), "Bearer")

    def test_llm_meta_preserves_minimax_provider_identity(self):
        llm = _llm_meta(
            "MiniMax-M3",
            "summary",
            {"input_tokens": 0, "output_tokens": 0},
        )

        self.assertEqual(llm.get("provider"), "minimax")
        self.assertEqual(llm.get("model"), "MiniMax-M3")
        self.assertEqual(llm.get("pricing_model_family"), "MiniMax-M3")
        self.assertEqual(llm.get("pricing_source"), "minimax_docs_2026_10_21")

    @patch("app.services.minimax_service._p._chat_json")
    def test_summarize_keeps_taxonomy_genre_from_structured_output(self, chat_json):
        chat_json.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 10, "output_tokens": 20},
        )

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="MiniMax-M2.5",
            api_key="minimax-key",
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")

    @patch("app.services.minimax_service._p._chat_json_async")
    def test_summarize_async_keeps_taxonomy_genre_from_structured_output(self, chat_json):
        chat_json.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 10, "output_tokens": 20},
        )

        result = asyncio.run(
            summarize_async(
                title="Example title",
                facts=["Fact 1"],
                source_text_chars=1200,
                model="MiniMax-M2.5",
                api_key="minimax-key",
            )
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
