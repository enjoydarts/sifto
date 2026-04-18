import unittest
from unittest.mock import patch

from app.services.llm_catalog import provider_api_key_header, provider_for_model
from app.services.xiaomi_mimo_token_plan_service import _chat_completions_url, summarize


class XiaomiMiMoTokenPlanServiceTests(unittest.TestCase):
    def test_provider_for_model_detects_xiaomi_mimo_token_plan(self):
        self.assertEqual(provider_for_model("mimo-v2-pro"), "xiaomi_mimo_token_plan")
        self.assertEqual(provider_for_model("mimo-v2-omni"), "xiaomi_mimo_token_plan")

    def test_provider_api_key_header_uses_openai_compatible_internal_header(self):
        self.assertEqual(provider_api_key_header("xiaomi_mimo_token_plan"), "x-openai-api-key")

    def test_chat_completions_url_normalizes_base(self):
        self.assertEqual(_chat_completions_url("https://token-plan-sgp.xiaomimimo.com/v1"), "https://token-plan-sgp.xiaomimimo.com/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://token-plan-sgp.xiaomimimo.com/v1/chat/completions"), "https://token-plan-sgp.xiaomimimo.com/v1/chat/completions")

    @patch("app.services.xiaomi_mimo_token_plan_service._p._chat_json")
    def test_summarize_preserves_genre(self, chat_json):
        chat_json.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"ai","other_label":"","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 1, "output_tokens": 1},
        )

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="mimo-v2-pro",
            api_key="mimo-key",
        )

        self.assertEqual(result["genre"], "ai")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
