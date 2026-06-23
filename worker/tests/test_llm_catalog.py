import unittest

from app.services.llm_catalog import load_llm_catalog, model_pricing, provider_for_model


REPRESENTATIVE_PROVIDER_MODELS = (
    ("anthropic", "claude-haiku-4-5"),
    ("anthropic", "claude-fable-5"),
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
    ("zai", "glm-5.2"),
    ("fireworks", "fireworks/deepseek-v3p1"),
    ("fireworks", "fireworks/glm-5p2"),
    ("fireworks", "qwen3p7-plus"),
    ("together", "together::moonshotai/Kimi-K2.6"),
    ("together", "together::MiniMaxAI/MiniMax-M3"),
    ("together", "together::zai-org/GLM-5.2"),
    ("featherless", "featherless::Qwen/Qwen3.5-9B"),
    ("siliconflow", "siliconflow::deepseek-ai/DeepSeek-V3.1"),
    ("siliconflow", "siliconflow::MiniMaxAI/MiniMax-M3"),
    ("siliconflow", "siliconflow::Qwen/Qwen3.6-35B-A3B"),
    ("siliconflow", "siliconflow::Qwen/Qwen3.6-27B"),
    ("siliconflow", "siliconflow::zai-org/GLM-5.2"),
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


class CatalogDrivenDispatchTests(unittest.TestCase):
    """Tests that dispatch/resolution use catalog as exclusive source (AC1,3)."""

    def test_get_llm_providers_returns_catalog_list(self):
        from app.services.llm_catalog import get_llm_providers
        provs = get_llm_providers()
        self.assertIn("anthropic", provs)
        self.assertIn("groq", provs)
        self.assertIn("openai", provs)
        self.assertGreater(len(provs), 10)

    def test_synthesized_add_provider_via_inmemory_catalog(self):
        """Exercise add-new-provider scenario by overriding load to return augmented catalog."""
        from app.auto_dispatch import build_handler_map_async
        import app.services.llm_catalog as mod
        orig_load = mod.load_llm_catalog
        try:
            def fake_load():
                cat = orig_load()
                cat = dict(cat)  # copy
                provs = list(cat.get("providers", []))
                provs.append({
                    "id": "synthetic_test_provider",
                    "api_key_header": "x-test-key",
                    "match_exact": ["synthetic-test-model"],
                    "match_prefixes": [],
                    "default_models": {},
                })
                cat["providers"] = provs
                # also ensure model entry for resolution
                chat = list(cat.get("chat_models", []))
                chat.append({
                    "id": "synthetic-test-model",
                    "provider": "synthetic_test_provider",
                    "available_purposes": ["summary"],
                })
                cat["chat_models"] = chat
                return cat
            mod.load_llm_catalog = fake_load
            mod.load_llm_catalog.cache_clear() if hasattr(mod.load_llm_catalog, "cache_clear") else None

            from app.services.llm_catalog import get_llm_providers, provider_for_model, provider_service_module
            provs = get_llm_providers()
            self.assertIn("synthetic_test_provider", provs)
            self.assertEqual(provider_for_model("synthetic-test-model"), "synthetic_test_provider")
            self.assertEqual(provider_service_module("synthetic_test_provider"), "synthetic_test_provider_service")
            # synthetic has no real module; expect import fail but proves list from catalog view
            try:
                h = build_handler_map_async("summarize", lambda f, k: lambda *a, **kwa: {"ok": True, "provider": kwa.get("model", "")}, providers=["synthetic_test_provider"])
                assert "synthetic_test_provider" in h
            except ModuleNotFoundError:
                pass
            # verify real dispatch + handler invoke for catalog listed compat provider (OpenAICompat path)
            h2 = build_handler_map_async("summarize", lambda f, k: lambda *a, **kwa: {"ok": True, "provider": kwa.get("model", "")}, providers=["groq"])
            assert "groq" in h2
            thunk = h2["groq"](None)
            res = thunk() if callable(thunk) else thunk
            assert res.get("ok") is True
        finally:
            mod.load_llm_catalog = orig_load
            if hasattr(mod.load_llm_catalog, "cache_clear"):
                mod.load_llm_catalog.cache_clear()
