import unittest

from app.services.llm_catalog import provider_for_model
from app.services.moonshot_service import _llm_meta


class MoonshotServiceTests(unittest.TestCase):
    def test_provider_for_model_detects_moonshot(self):
        self.assertEqual(provider_for_model("kimi-k2.5"), "moonshot")
        self.assertEqual(provider_for_model("kimi-k2-0905-preview"), "moonshot")
        self.assertEqual(provider_for_model("kimi-k2-thinking-turbo"), "moonshot")

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

    def test_llm_meta_uses_static_catalog_pricing(self):
        llm = _llm_meta(
            "kimi-k2-0905-preview",
            "summary",
            {
                "input_tokens": 1_000_000,
                "output_tokens": 1_000_000,
                "cache_read_input_tokens": 0,
            },
        )

        self.assertEqual(llm.get("pricing_source"), "moonshot_static_2026_03")
        self.assertEqual(llm.get("pricing_model_family"), "kimi-k2-0905-preview")
        self.assertEqual(llm.get("estimated_cost_usd"), 3.1)


if __name__ == "__main__":
    unittest.main()
