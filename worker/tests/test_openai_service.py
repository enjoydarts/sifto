import asyncio
import unittest
from unittest.mock import AsyncMock, patch

from app.services.openai_service import _p


class OpenAIServiceModelBehaviorTests(unittest.TestCase):
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
