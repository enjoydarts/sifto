import unittest
from unittest.mock import patch

from app.services.gemini_service import summarize


class GeminiServiceTests(unittest.TestCase):
    @patch("app.services.gemini_service._summary_context_cache_enabled", return_value=False)
    @patch("app.services.gemini_service._generate_content")
    def test_summarize_keeps_taxonomy_genre_from_structured_output(self, generate_content, _summary_context_cache_enabled):
        generate_content.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 10, "output_tokens": 20, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0},
        )

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="gemini-2.5-flash",
            api_key="test-key",
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
