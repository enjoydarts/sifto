import unittest
from unittest.mock import patch

from app.services.llm_catalog import provider_api_key_header, provider_for_model, resolve_model_id


class FeatherlessCatalogTests(unittest.TestCase):
    def test_provider_for_model_detects_featherless_alias(self):
        self.assertEqual(provider_for_model("featherless::Qwen/Qwen3.5-9B"), "featherless")

    def test_resolve_model_id_strips_featherless_alias(self):
        self.assertEqual(resolve_model_id("featherless::Qwen/Qwen3.5-9B"), "Qwen/Qwen3.5-9B")

    def test_provider_api_key_header_uses_openai_compatible_internal_header(self):
        self.assertEqual(provider_api_key_header("featherless"), "x-openai-api-key")


class FeatherlessServiceTests(unittest.TestCase):
    def test_chat_completions_url_normalizes_base(self):
        from app.services.featherless_service import _chat_completions_url

        self.assertEqual(_chat_completions_url("https://api.featherless.ai/v1"), "https://api.featherless.ai/v1/chat/completions")
        self.assertEqual(
            _chat_completions_url("https://api.featherless.ai/v1/chat/completions"),
            "https://api.featherless.ai/v1/chat/completions",
        )

    @patch("app.services.featherless_service._p._chat_json")
    def test_summarize_preserves_genre_from_structured_output(self, chat_json):
        from app.services.featherless_service import summarize

        chat_json.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"ai","other_label":"","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 1, "output_tokens": 1},
        )

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="featherless::Qwen/Qwen3.5-9B",
            api_key="featherless-key",
        )

        self.assertEqual(result["genre"], "ai")
        self.assertEqual(result["other_label"], "")

    def test_moonshot_kimi_k25_uses_native_moonshot_sampling_defaults(self):
        from app.services.featherless_service import _normalize_temperature, _normalize_top_p

        self.assertEqual(_normalize_temperature("moonshotai/Kimi-K2.5", None), 0.6)
        self.assertEqual(_normalize_temperature("moonshotai/Kimi-K2.5", 0.2), 0.6)
        self.assertEqual(_normalize_temperature("moonshotai/Kimi-K2.5", 1.0), 0.6)
        self.assertEqual(_normalize_top_p("moonshotai/Kimi-K2.5", None), 0.95)
        self.assertEqual(_normalize_top_p("moonshotai/Kimi-K2.5", 0.8), 0.95)

    def test_non_moonshot_models_keep_caller_sampling(self):
        from app.services.featherless_service import _normalize_temperature, _normalize_top_p

        self.assertEqual(_normalize_temperature("Qwen/Qwen3.5-9B", 0.4), 0.4)
        self.assertIsNone(_normalize_top_p("Qwen/Qwen3.5-9B", None))
        self.assertEqual(_normalize_top_p("Qwen/Qwen3.5-9B", 0.9), 0.9)


if __name__ == "__main__":
    unittest.main()
