import asyncio
import unittest
from unittest.mock import patch

from app.services.claude_service import _llm_meta, summarize, summarize_async


class ClaudeServiceTests(unittest.TestCase):
    def test_llm_meta_prices_opus_4_7_family(self):
        class _Message:
            model = "claude-opus-4-7"
            usage = type(
                "Usage",
                (),
                {
                    "input_tokens": 1_000_000,
                    "output_tokens": 500_000,
                    "cache_creation_input_tokens": 0,
                    "cache_read_input_tokens": 0,
                },
            )()

        llm = _llm_meta(_Message(), "summary", "claude-opus-4-7")
        self.assertEqual(llm["pricing_model_family"], "claude-opus-4-7")
        self.assertEqual(llm["pricing_source"], "anthropic_static_2026_04")
        self.assertEqual(llm["estimated_cost_usd"], 17.5)

    def test_summarize_requires_model(self):
        with self.assertRaisesRegex(RuntimeError, "anthropic model is required for summary"):
            summarize(
                title="title",
                facts=["fact"],
                api_key="anthropic-key",
                model=None,
            )

    def test_summarize_async_requires_model(self):
        with self.assertRaisesRegex(RuntimeError, "anthropic model is required for summary"):
            asyncio.run(
                summarize_async(
                    title="title",
                    facts=["fact"],
                    api_key="anthropic-key",
                    model=None,
                )
            )

    def test_summarize_requires_api_key(self):
        with self.assertRaisesRegex(RuntimeError, "anthropic api key is required for summary"):
            summarize(
                title="title",
                facts=["fact"],
                model="claude-sonnet-4-6",
                api_key=None,
            )

    def test_summarize_async_requires_api_key(self):
        with self.assertRaisesRegex(RuntimeError, "anthropic api key is required for summary"):
            asyncio.run(
                summarize_async(
                    title="title",
                    facts=["fact"],
                    model="claude-sonnet-4-6",
                    api_key=None,
                )
            )

    @patch("app.services.claude_service._llm_meta", return_value={"provider": "anthropic", "model": "claude-sonnet-4-6"})
    @patch("app.services.claude_service._message_text")
    @patch("app.services.claude_service._call_with_model_fallback")
    @patch("app.services.claude_service._client_for_api_key", return_value=object())
    def test_summarize_keeps_taxonomy_genre_from_structured_output(self, _client_for_api_key, call_with_model_fallback, message_text, _llm_meta):
        call_with_model_fallback.return_value = (object(), "claude-sonnet-4-6", [])
        message_text.return_value = '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}'

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="claude-sonnet-4-6",
            api_key="anthropic-key",
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
