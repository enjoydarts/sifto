import os
import unittest
from unittest.mock import patch

from app.services.provider_base import OpenAICompatProvider, ProviderConfig
from app.services.provider_pricing import (
    estimate_cost_usd,
    normalize_model_family,
    normalize_model_name,
    pricing_for_model,
)


class ProviderPricingTests(unittest.TestCase):
    def test_normalize_model_name_can_resolve_catalog_alias(self):
        self.assertEqual(
            normalize_model_name(" openrouter::anthropic/claude-sonnet-4-5 ", use_resolve_model_id=True),
            "anthropic/claude-sonnet-4-5",
        )

    def test_normalize_model_family_keeps_catalog_model_before_legacy_prefix(self):
        with patch("app.services.provider_pricing.model_pricing", return_value={"input_per_mtok_usd": 1.0}):
            self.assertEqual(
                normalize_model_family(
                    "legacy-model-v2",
                    legacy_model_pricing={"legacy-model": {"input_per_mtok_usd": 9.0}},
                ),
                "legacy-model-v2",
            )

    def test_normalize_model_family_uses_longest_legacy_prefix(self):
        with patch("app.services.provider_pricing.model_pricing", return_value=None):
            self.assertEqual(
                normalize_model_family(
                    "legacy-model-large-v2",
                    legacy_model_pricing={
                        "legacy-model": {"input_per_mtok_usd": 1.0},
                        "legacy-model-large": {"input_per_mtok_usd": 2.0},
                    },
                ),
                "legacy-model-large",
            )

    def test_pricing_for_model_applies_legacy_fallback_and_env_overrides(self):
        env = {
            "TEST_PROVIDER_SUMMARY_INPUT_PER_MTOK_USD": "3.5",
            "TEST_PROVIDER_SUMMARY_CACHE_READ_PER_MTOK_USD": "0.25",
        }
        with patch("app.services.provider_pricing.model_pricing", return_value=None), patch.dict(os.environ, env, clear=False):
            pricing = pricing_for_model(
                "legacy-model-v2",
                "summary",
                env_prefix="TEST_PROVIDER",
                pricing_source_version="default_source",
                legacy_model_pricing={
                    "legacy-model": {
                        "input_per_mtok_usd": 1.0,
                        "output_per_mtok_usd": 8.0,
                        "cache_read_per_mtok_usd": 0.1,
                    }
                },
                normalize_model_family_func=lambda _: "legacy-model",
            )

        self.assertEqual(pricing["pricing_model_family"], "legacy-model")
        self.assertEqual(pricing["pricing_source"], "env_override")
        self.assertEqual(pricing["input_per_mtok_usd"], 3.5)
        self.assertEqual(pricing["output_per_mtok_usd"], 8.0)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.25)

    def test_pricing_for_model_returns_zero_rates_for_unknown_model(self):
        with patch("app.services.provider_pricing.model_pricing", return_value=None):
            pricing = pricing_for_model(
                "unknown-model",
                "summary",
                env_prefix="TEST_PROVIDER",
                pricing_source_version="default_source",
                normalize_model_family_func=lambda model: model,
            )

        self.assertEqual(pricing["pricing_model_family"], "unknown-model")
        self.assertEqual(pricing["pricing_source"], "default_source")
        self.assertEqual(pricing["input_per_mtok_usd"], 0.0)
        self.assertEqual(pricing["output_per_mtok_usd"], 0.0)
        self.assertEqual(pricing["cache_read_per_mtok_usd"], 0.0)

    def test_estimate_cost_usd_preserves_cache_read_behavior(self):
        cost = estimate_cost_usd(
            "model",
            "summary",
            {
                "input_tokens": 1000,
                "output_tokens": 200,
                "cache_creation_input_tokens": 9999,
                "cache_read_input_tokens": 400,
            },
            pricing_for_model_func=lambda *_: {
                "input_per_mtok_usd": 2.0,
                "output_per_mtok_usd": 10.0,
                "cache_read_per_mtok_usd": 0.5,
            },
        )

        self.assertEqual(cost, 0.0034)

    def test_provider_resolve_cost_prefers_billed_cost_when_enabled(self):
        provider = OpenAICompatProvider(
            ProviderConfig(
                provider_name="test_provider",
                env_prefix="TEST_PROVIDER",
                pricing_source_version="default_source",
                api_base_url="https://example.com",
                api_base_url_env="TEST_PROVIDER_BASE_URL",
                use_billed_cost=True,
            )
        )

        cost = provider._resolve_cost(
            "model",
            "summary",
            {"input_tokens": 1000, "output_tokens": 1000, "billed_cost_usd": 0.1234},
        )

        self.assertEqual(cost, 0.1234)

    def test_provider_meta_shape_is_unchanged_after_pricing_extraction(self):
        provider = OpenAICompatProvider(
            ProviderConfig(
                provider_name="test_provider",
                env_prefix="TEST_PROVIDER",
                pricing_source_version="default_source",
                api_base_url="https://example.com",
                api_base_url_env="TEST_PROVIDER_BASE_URL",
                legacy_model_pricing={
                    "legacy-model": {
                        "input_per_mtok_usd": 2.0,
                        "output_per_mtok_usd": 10.0,
                        "cache_read_per_mtok_usd": 0.5,
                    }
                },
            )
        )

        with patch("app.services.provider_pricing.model_pricing", return_value=None):
            meta = provider._llm_meta(
                "legacy-model-v2",
                "summary",
                {"input_tokens": 1000, "output_tokens": 200, "cache_read_input_tokens": 400},
            )

        self.assertEqual(
            meta,
            {
                "provider": "test_provider",
                "model": "legacy-model-v2",
                "pricing_model_family": "legacy-model",
                "pricing_source": "default_source",
                "input_tokens": 1000,
                "output_tokens": 200,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 400,
                "estimated_cost_usd": 0.0034,
            },
        )


if __name__ == "__main__":
    unittest.main()
