import asyncio
import os
import unittest
from unittest.mock import patch

from app.services.llm_catalog import provider_api_key_header, provider_for_model
from app.services.minimax_service import (
    _chat_completions_url,
    _p,
    _llm_meta,
    _require_model,
    summarize,
    summarize_async,
)


class MiniMaxServiceTests(unittest.TestCase):
    def test_provider_for_model_detects_minimax(self):
        self.assertEqual(provider_for_model("MiniMax-M2.5"), "minimax")
        self.assertEqual(provider_for_model("MiniMax-M2.7"), "minimax")
        self.assertEqual(provider_for_model("minimax::MiniMax-M2.7"), "minimax")
        self.assertEqual(provider_for_model("minimax/MiniMax-M2.7"), "minimax")

    def test_provider_api_key_header_uses_minimax_header(self):
        self.assertEqual(provider_api_key_header("minimax"), "x-minimax-api-key")

    def test_model_is_required(self):
        with self.assertRaisesRegex(RuntimeError, "minimax model is required for summary"):
            _require_model(None, "summary")

    def test_chat_completions_url_defaults_to_openai_compatible_endpoint(self):
        self.assertEqual(_chat_completions_url(""), "https://api.minimax.io/v1/chat/completions")

    def test_chat_completions_url_normalizes_env_override(self):
        self.assertEqual(_chat_completions_url("https://api.minimax.io"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/v1"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/v1/chat/completions"), "https://api.minimax.io/v1/chat/completions")

    @patch("app.services.provider_base.run_chat_json", return_value=("{}", {"input_tokens": 1, "output_tokens": 2}))
    def test_chat_json_uses_minimax_openai_compatible_url(self, chat_json):
        with patch.dict(os.environ, {"MINIMAX_API_BASE_URL": "https://api.minimax.io"}, clear=False):
            _p._chat_json("prompt", "MiniMax-M2.7", "minimax-key", max_output_tokens=128)
        self.assertEqual(chat_json.call_args.kwargs.get("url"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(chat_json.call_args.kwargs.get("provider_name"), "minimax")
        self.assertEqual(chat_json.call_args.kwargs.get("auth_header_name"), "Authorization")
        self.assertEqual(chat_json.call_args.kwargs.get("auth_scheme"), "Bearer")

    def test_llm_meta_preserves_minimax_provider_identity(self):
        llm = _llm_meta(
            "MiniMax-M2.5",
            "summary",
            {"input_tokens": 0, "output_tokens": 0},
        )

        self.assertEqual(llm.get("provider"), "minimax")
        self.assertEqual(llm.get("model"), "MiniMax-M2.5")

    @patch("app.services.minimax_service._p._chat_json")
    def test_summarize_keeps_taxonomy_genre_from_structured_output(self, chat_json):
        chat_json.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 10, "output_tokens": 20},
        )

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="MiniMax-M2.5",
            api_key="minimax-key",
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")

    @patch("app.services.minimax_service._p._chat_json_async")
    def test_summarize_async_keeps_taxonomy_genre_from_structured_output(self, chat_json):
        chat_json.return_value = (
            '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}',
            {"input_tokens": 10, "output_tokens": 20},
        )

        result = asyncio.run(
            summarize_async(
                title="Example title",
                facts=["Fact 1"],
                source_text_chars=1200,
                model="MiniMax-M2.5",
                api_key="minimax-key",
            )
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
