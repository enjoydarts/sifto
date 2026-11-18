import asyncio
import json
import unittest
from unittest.mock import AsyncMock, patch

import httpx

from app.services.openai_service import _p


class OpenAIServiceModelBehaviorTests(unittest.TestCase):
    def test_gpt_6_2_strict_responses_and_usage(self):
        schema = {
            "type": "object",
            "properties": {"summary": {"type": "string"}},
            "required": ["summary"],
            "additionalProperties": False,
        }
        for model, short_cost, long_cost in (
            ("gpt-6.2-astra", 1.856, 3.512016),
            ("gpt-6.2-sol", 0.464, 0.878004),
        ):
            for asynchronous, input_tokens, expected_cost in (
                (False, 272000, short_cost),
                (True, 272001, long_cost),
            ):
                with self.subTest(model=model, asynchronous=asynchronous):
                    response = httpx.Response(200, json={
                        "output_text": '{"summary":"ok"}',
                        "usage": {
                            "input_tokens": input_tokens,
                            "input_tokens_details": {"cached_tokens": 100000, "cache_write_tokens": 20000},
                            "output_tokens": 10000,
                        },
                    })
                    client = httpx.AsyncClient if asynchronous else httpx.Client
                    with patch.object(client, "post", return_value=response) as post:
                        kwargs = {
                            "system_instruction": "Summarize the article.",
                            "response_schema": schema,
                            "temperature": 0.2,
                            "top_p": 0.9,
                        }
                        if asynchronous:
                            result, usage = asyncio.run(_p._chat_json_async("Article", model, "api-key", **kwargs))
                        else:
                            result, usage = _p._chat_json("Article", model, "api-key", **kwargs)

                    self.assertEqual(json.loads(result), {"summary": "ok"})
                    post.assert_called_once()
                    self.assertEqual(post.call_args.args[0], "https://api.openai.com/v1/responses")
                    body = post.call_args.kwargs["json"]
                    self.assertEqual(body["model"], model)
                    self.assertEqual(body["reasoning"], {"effort": "low"})
                    self.assertEqual(body["input"], [
                        {"role": "developer", "content": "Summarize the article."},
                        {"role": "user", "content": "Article"},
                    ])
                    for key in ("instructions", "temperature", "top_p"):
                        self.assertNotIn(key, body)
                    self.assertEqual(body["text"]["format"], {
                        "type": "json_schema", "name": "response", "schema": schema, "strict": True,
                    })
                    meta = _p._llm_meta(model, "summary", usage)
                    self.assertEqual(meta["pricing_model_family"], model)
                    self.assertEqual(meta["cache_creation_input_tokens"], 20000)
                    self.assertEqual(meta["cache_read_input_tokens"], 100000)
                    self.assertEqual(meta["estimated_cost_usd"], expected_cost)

    def test_gpt_6_1_terra_uses_strict_responses_without_sampling(self):
        model = "gpt-6.1-terra"
        self.assertTrue(_p._should_use_responses_api(model))
        self.assertTrue(_p._supports_strict_schema(model))
        self.assertEqual(_p._responses_reasoning(model), {"effort": "none"})
        schema = {"type": "object", "properties": {"summary": {"type": "string"}}, "required": ["summary"]}
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                target = "app.services.openai_service.run_responses_json" + ("_async" if asynchronous else "")
                mock_type = AsyncMock if asynchronous else None
                with patch(target, new_callable=mock_type, return_value=('{}', {})) as run:
                    kwargs = {"temperature": 0.2, "top_p": 0.9, "response_schema": schema}
                    if asynchronous:
                        asyncio.run(_p._chat_json_async("prompt", model, "api-key", **kwargs))
                    else:
                        _p._chat_json("prompt", model, "api-key", **kwargs)
                self.assertEqual(run.call_args.args[1], model)
                self.assertEqual(run.call_args.kwargs["response_schema"], schema)
                self.assertTrue(run.call_args.kwargs["supports_strict_schema"](model))
                self.assertEqual(run.call_args.kwargs["responses_reasoning"](model), {"effort": "none"})
                self.assertNotIn("temperature", run.call_args.kwargs)
                self.assertNotIn("top_p", run.call_args.kwargs)

    def test_gpt_6_1_terra_prices_cache_and_long_context_at_boundary(self):
        cases = (
            (272000, 0.227),
            (272001, 0.429002),
        )
        for input_tokens, expected in cases:
            with self.subTest(input_tokens=input_tokens):
                meta = _p._llm_meta("gpt-6.1-terra", "summary", {
                    "input_tokens": input_tokens,
                    "cache_read_input_tokens": 100000,
                    "output_tokens": 10000,
                })
                self.assertEqual(meta["provider"], "openai")
                self.assertEqual(meta["pricing_model_family"], "gpt-6.1-terra")
                self.assertEqual(meta["pricing_source"], "openai_model_pages_2026_10")
                self.assertEqual(meta["estimated_cost_usd"], expected)

    def test_gpt_6_1_sol_uses_low_reasoning_without_sampling_parameters(self):
        self.assertTrue(_p._should_use_responses_api("gpt-6.1-sol"))
        self.assertFalse(_p._supports_custom_temperature("gpt-6.1-sol"))
        self.assertTrue(_p._supports_strict_schema("gpt-6.1-sol"))
        self.assertEqual(_p._responses_reasoning("gpt-6.1-sol"), {"effort": "low"})

        with patch("app.services.openai_service.run_responses_json", return_value=("{}", {})) as run:
            _p._chat_json("prompt", "gpt-6.1-sol", "api-key", temperature=0.2, top_p=0.9)

        self.assertEqual(run.call_args.args[1], "gpt-6.1-sol")
        self.assertNotIn("temperature", run.call_args.kwargs)
        self.assertNotIn("top_p", run.call_args.kwargs)

    def test_gpt_6_1_sol_async_uses_responses_api(self):
        with patch("app.services.openai_service.run_responses_json_async", new_callable=AsyncMock, return_value=("{}", {})) as run:
            asyncio.run(_p._chat_json_async("prompt", "gpt-6.1-sol", "api-key", temperature=0.2, top_p=0.9))

        kwargs = run.call_args.kwargs
        self.assertEqual(kwargs["responses_reasoning"]("gpt-6.1-sol"), {"effort": "low"})
        self.assertNotIn("temperature", kwargs)
        self.assertNotIn("top_p", kwargs)

    def test_gpt_6_1_sol_records_cached_input_cost(self):
        usage = _p._llm_meta("gpt-6.1-sol", "summary", {
            "input_tokens": 1000000, "output_tokens": 1000000,
            "cache_read_input_tokens": 500000,
        })

        self.assertEqual(usage["model"], "gpt-6.1-sol")
        self.assertEqual(usage["pricing_source"], "openai_model_pages_2026_10")
        self.assertEqual(usage["estimated_cost_usd"], 11.05)

    def test_gpt_6_sol_and_luna_use_responses_api_without_sampling_parameters(self):
        for model in ("gpt-6-sol", "gpt-6-luna"):
            with self.subTest(model=model):
                self.assertTrue(_p._should_use_responses_api(model))
                self.assertFalse(_p._supports_custom_temperature(model))
                self.assertEqual(_p._responses_reasoning(model), {"effort": "none"})

    def test_gpt_6_astra_uses_responses_api_without_sampling_parameters(self):
        self.assertTrue(_p._should_use_responses_api("gpt-6-astra"))
        self.assertFalse(_p._supports_custom_temperature("gpt-6-astra"))

        with patch("app.services.openai_service.run_responses_json", return_value=("{}", {})) as run:
            _p._responses_json(
                "prompt",
                "gpt-6-astra",
                "api-key",
                temperature=0.2,
                top_p=0.9,
            )

        kwargs = run.call_args.kwargs
        self.assertNotIn("temperature", kwargs)
        self.assertNotIn("top_p", kwargs)

    def test_gpt_6_astra_uses_low_reasoning_effort(self):
        self.assertEqual(_p._responses_reasoning("gpt-6-astra"), {"effort": "low"})


if __name__ == "__main__":
    unittest.main()
