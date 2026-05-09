import unittest

from app.services.llm_catalog import model_pricing, model_supports, provider_for_model


class GoogleCatalogTests(unittest.TestCase):
    def test_gemini_31_flash_lite_stable_is_available(self):
        pricing = model_pricing("gemini-3.1-flash-lite")

        self.assertEqual(provider_for_model("gemini-3.1-flash-lite"), "google")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.25)
        self.assertEqual(pricing["output_per_mtok_usd"], 1.5)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.025)
        self.assertTrue(model_supports("gemini-3.1-flash-lite", "supports_structured_output"))


if __name__ == "__main__":
    unittest.main()
