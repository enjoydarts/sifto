import unittest

from app.services.deepseek_service import _p
from app.services.llm_catalog import model_pricing, model_supports, provider_for_model


class DeepSeekCatalogTests(unittest.TestCase):
    def test_deepseek_peak_pricing(self):
        expected_by_model = {
            "deepseek-v4-flash": (0.44, 1.32, 0.014),
            "deepseek-v4-pro": (1.32, 3.96, 0.044),
            "deepseek-chat": (0.44, 1.32, 0.014),
            "deepseek-reasoner": (0.44, 1.32, 0.014),
        }

        for model, (input_price, output_price, cache_read_price) in expected_by_model.items():
            with self.subTest(model=model):
                pricing = model_pricing(model)

                self.assertEqual(provider_for_model(model), "deepseek")
                self.assertIsNotNone(pricing)
                self.assertEqual(pricing["pricing_source"], "deepseek_pricing_2026_08_24_peak")
                self.assertEqual(pricing["input_per_mtok_usd"], input_price)
                self.assertEqual(pricing["output_per_mtok_usd"], output_price)
                self.assertEqual(pricing["cache_read_per_mtok_usd"], cache_read_price)
                self.assertNotIn("cache_write_per_mtok_usd", pricing)

    def test_deepseek_cache_pricing_capabilities(self):
        for model in ("deepseek-v4-flash", "deepseek-v4-pro", "deepseek-chat", "deepseek-reasoner"):
            with self.subTest(model=model):
                self.assertTrue(model_supports(model, "supports_cache_read_pricing"))
                self.assertFalse(model_supports(model, "supports_cache_write_pricing"))

    def test_deepseek_provider_pricing_source_fallback(self):
        self.assertEqual(_p.config.pricing_source_version, "deepseek_pricing_2026_08_24_peak")


if __name__ == "__main__":
    unittest.main()
