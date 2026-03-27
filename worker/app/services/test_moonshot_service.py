import unittest

from app.services.llm_catalog import provider_for_model
from app.services.moonshot_service import _llm_meta, _normalize_temperature, _normalize_top_p


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

    def test_temperature_is_forced_to_one(self):
        self.assertEqual(_normalize_temperature("kimi-k2-thinking", None), 1.0)
        self.assertEqual(_normalize_temperature("kimi-k2-thinking", 0.2), 1.0)
        self.assertEqual(_normalize_temperature("kimi-k2-thinking", 1.0), 1.0)

    def test_kimi_k25_temperature_is_forced_to_point_six(self):
        self.assertEqual(_normalize_temperature("kimi-k2.5", None), 0.6)
        self.assertEqual(_normalize_temperature("kimi-k2.5", 0.2), 0.6)
        self.assertEqual(_normalize_temperature("kimi-k2.5", 1.0), 0.6)

    def test_top_p_is_forced_to_point_ninety_five(self):
        self.assertEqual(_normalize_top_p(None), 0.95)
        self.assertEqual(_normalize_top_p(0.8), 0.95)
        self.assertEqual(_normalize_top_p(0.95), 0.95)


if __name__ == "__main__":
    unittest.main()
