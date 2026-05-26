import unittest

from app.services.llm_catalog import model_pricing, model_supports, provider_for_model


class DeepSeekCatalogTests(unittest.TestCase):
    def test_deepseek_v4_pro_pricing(self):
        pricing = model_pricing("deepseek-v4-pro")

        self.assertEqual(provider_for_model("deepseek-v4-pro"), "deepseek")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.435)
        self.assertEqual(pricing["output_per_mtok_usd"], 0.87)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.003625)
        self.assertNotIn("cache_write_per_mtok_usd", pricing)
        self.assertTrue(model_supports("deepseek-v4-pro", "supports_cache_read_pricing"))
        self.assertFalse(model_supports("deepseek-v4-pro", "supports_cache_write_pricing"))


if __name__ == "__main__":
    unittest.main()
