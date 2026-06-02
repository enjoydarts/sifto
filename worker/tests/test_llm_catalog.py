import unittest

from app.services.llm_catalog import load_llm_catalog, model_pricing, provider_for_model


REPRESENTATIVE_PROVIDER_MODELS = (
    ("anthropic", "claude-haiku-4-5"),
    ("anthropic", "claude-opus-4-8"),
    ("google", "gemini-3.1-flash-lite"),
    ("groq", "openai/gpt-oss-20b"),
    ("deepseek", "deepseek-chat"),
    ("alibaba", "qwen3.7-max"),
    ("mistral", "mistral-small-2603"),
    ("cerebras", "gpt-oss-120b"),
    ("minimax", "MiniMax-M3"),
    ("xiaomi_mimo_token_plan", "mimo-v2-pro"),
    ("moonshot", "kimi-k2.6"),
    ("xai", "grok-4"),
    ("zai", "glm-5.1"),
    ("fireworks", "fireworks/deepseek-v3p1"),
    ("together", "together::moonshotai/Kimi-K2.6"),
    ("featherless", "featherless::Qwen/Qwen3.5-9B"),
    ("siliconflow", "siliconflow::deepseek-ai/DeepSeek-V3.1"),
    ("siliconflow", "siliconflow::MiniMaxAI/MiniMax-M3"),
    ("siliconflow", "siliconflow::Qwen/Qwen3.6-35B-A3B"),
    ("siliconflow", "siliconflow::Qwen/Qwen3.6-27B"),
    ("siliconflow", "siliconflow::zai-org/GLM-5.1"),
    ("openai", "gpt-5-mini"),
)

ALIASED_PROVIDER_MODELS = (
    ("cerebras", "cerebras::gpt-oss-120b"),
    ("minimax", "minimax::MiniMax-M2.7"),
    ("minimax", "minimax/MiniMax-M2.7"),
)


class LlmCatalogSmokeTests(unittest.TestCase):
    def test_provider_match_rules_are_arrays(self):
        catalog = load_llm_catalog()
        for provider in catalog.get("providers", []):
            with self.subTest(provider=provider.get("id")):
                self.assertIsInstance(provider.get("match_exact"), list)
                self.assertIsInstance(provider.get("match_prefixes"), list)

    def test_provider_match_rules_resolve_to_provider(self):
        catalog = load_llm_catalog()
        for provider in catalog.get("providers", []):
            provider_id = str(provider.get("id") or "").strip()
            for exact in provider.get("match_exact", []):
                model = f"together::{exact}" if provider_id == "together" else exact
                with self.subTest(provider=provider_id, model=model):
                    self.assertEqual(provider_for_model(model), provider_id)
            for prefix in provider.get("match_prefixes", []):
                model = f"{prefix}catalog-smoke-test"
                with self.subTest(provider=provider_id, model=model):
                    self.assertEqual(provider_for_model(model), provider_id)

    def test_representative_provider_models_resolve_and_have_pricing(self):
        for expected_provider, model in REPRESENTATIVE_PROVIDER_MODELS:
            with self.subTest(provider=expected_provider, model=model):
                pricing = model_pricing(model)

                self.assertEqual(provider_for_model(model), expected_provider)
                self.assertIsNotNone(pricing)
                self.assertIn("input_per_mtok_usd", pricing)
                self.assertIn("output_per_mtok_usd", pricing)

    def test_prefixed_provider_aliases_resolve_and_have_pricing(self):
        for expected_provider, model in ALIASED_PROVIDER_MODELS:
            with self.subTest(provider=expected_provider, model=model):
                pricing = model_pricing(model)

                self.assertEqual(provider_for_model(model), expected_provider)
                self.assertIsNotNone(pricing)
                self.assertIn("input_per_mtok_usd", pricing)
                self.assertIn("output_per_mtok_usd", pricing)


if __name__ == "__main__":
    unittest.main()
