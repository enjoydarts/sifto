import unittest

from app.services.llm_catalog import model_pricing, model_supports, provider_for_model


class GoogleCatalogTests(unittest.TestCase):
    def test_gemini_36_flash_is_available(self):
        pricing = model_pricing("gemini-3.6-flash")

        self.assertEqual(provider_for_model("gemini-3.6-flash"), "google")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 1.5)
        self.assertEqual(pricing["output_per_mtok_usd"], 7.5)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.15)
        self.assertTrue(model_supports("gemini-3.6-flash", "supports_structured_output"))
        self.assertTrue(model_supports("gemini-3.6-flash", "supports_reasoning"))

    def test_gemini_35_flash_lite_is_available(self):
        pricing = model_pricing("gemini-3.5-flash-lite")

        self.assertEqual(provider_for_model("gemini-3.5-flash-lite"), "google")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.3)
        self.assertEqual(pricing["output_per_mtok_usd"], 2.5)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.03)
        self.assertTrue(model_supports("gemini-3.5-flash-lite", "supports_structured_output"))
        self.assertTrue(model_supports("gemini-3.5-flash-lite", "supports_reasoning"))

    def test_gemini_31_flash_lite_stable_is_available(self):
        pricing = model_pricing("gemini-3.1-flash-lite")

        self.assertEqual(provider_for_model("gemini-3.1-flash-lite"), "google")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 0.25)
        self.assertEqual(pricing["output_per_mtok_usd"], 1.5)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.025)
        self.assertTrue(model_supports("gemini-3.1-flash-lite", "supports_structured_output"))

    def test_gemini_35_flash_is_available(self):
        pricing = model_pricing("gemini-3.5-flash")

        self.assertEqual(provider_for_model("gemini-3.5-flash"), "google")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 1.5)
        self.assertEqual(pricing["output_per_mtok_usd"], 9.0)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.15)
        self.assertTrue(model_supports("gemini-3.5-flash", "supports_structured_output"))
        self.assertTrue(model_supports("gemini-3.5-flash", "supports_reasoning"))


if __name__ == "__main__":
    unittest.main()
