import unittest

from app.services.llm_catalog import provider_for_model
from app.services.moonshot_service import _llm_meta


class MoonshotServiceTests(unittest.TestCase):
    def test_provider_for_model_detects_moonshot(self):
        self.assertEqual(provider_for_model("kimi-k2.5"), "moonshot")

    def test_llm_meta_prefers_billed_cost_when_present(self):
        llm = _llm_meta(
            "kimi-k2.5",
            "summary",
            {
                "input_tokens": 120,
                "output_tokens": 80,
                "billed_cost_usd": 0.0123,
            },
        )

        self.assertEqual(llm.get("provider"), "moonshot")
        self.assertEqual(llm.get("model"), "kimi-k2.5")
        self.assertEqual(llm.get("estimated_cost_usd"), 0.0123)


if __name__ == "__main__":
    unittest.main()
