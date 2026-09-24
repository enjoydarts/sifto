import unittest
from unittest.mock import patch

from app.services.openai_service import _p


class OpenAIServiceModelBehaviorTests(unittest.TestCase):
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
