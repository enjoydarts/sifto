import unittest

from app.services.llm_catalog import model_pricing, model_supports, provider_for_model


class AlibabaCatalogTests(unittest.TestCase):
    def test_qwen37_max_is_available(self):
        pricing = model_pricing("qwen3.7-max")

        self.assertEqual(provider_for_model("qwen3.7-max"), "alibaba")
        self.assertIsNotNone(pricing)
        self.assertEqual(pricing["input_per_mtok_usd"], 2.5)
        self.assertEqual(pricing["output_per_mtok_usd"], 7.5)
        self.assertEqual(pricing["cache_write_per_mtok_usd"], 3.125)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.25)
        self.assertTrue(model_supports("qwen3.7-max", "supports_structured_output"))
        self.assertTrue(model_supports("qwen3.7-max", "supports_reasoning"))


if __name__ == "__main__":
    unittest.main()
