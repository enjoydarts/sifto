import unittest

from app.services.provider_base import OpenAICompatProvider, ProviderConfig


class _StubSummaryProvider(OpenAICompatProvider):
    def __init__(self):
        super().__init__(
            ProviderConfig(
                provider_name="test_provider",
                env_prefix="TEST_PROVIDER",
                pricing_source_version="test",
                api_base_url="https://example.com",
                api_base_url_env="TEST_PROVIDER_BASE_URL",
                default_model="test-model",
            )
        )

    def _chat_json(self, prompt, model, api_key, **kwargs):
        return (
            '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {},
        )

    def _translate_title_to_ja(self, raw_title, model, api_key):
        return raw_title

    def _llm_meta(self, model, purpose, usage):
        return {"provider": "test_provider", "model": model, "purpose": purpose}


class ProviderBaseSummaryTests(unittest.TestCase):
    def test_summarize_keeps_taxonomy_genre_in_final_result(self):
        provider = _StubSummaryProvider()

        result = provider.summarize(
            title="Example",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="test-model",
            api_key="test-key",
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
