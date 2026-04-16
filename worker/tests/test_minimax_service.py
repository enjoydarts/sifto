import unittest
from unittest.mock import patch

from app.services.llm_catalog import provider_api_key_header, provider_for_model
from app.services.minimax_service import (
    _api_base_url,
    _call_with_model_fallback,
    _call_with_model_fallback_async,
    _client_for_api_key,
    _async_client_for_api_key,
    _llm_meta,
    _require_model,
    summarize,
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

    @patch("app.services.minimax_service.os.getenv", return_value="https://api.minimax.io/custom-anthropic")
    def test_api_base_url_uses_env_override(self, getenv):
        self.assertEqual(_api_base_url(), "https://api.minimax.io/custom-anthropic")
        getenv.assert_called_with("MINIMAX_API_BASE_URL")

    @patch("app.services.minimax_service._transport_client_for_api_key", return_value=object())
    @patch("app.services.minimax_service.os.getenv", return_value="https://api.minimax.io/anthropic")
    def test_client_uses_minimax_anthropic_base_url(self, getenv, client_for_api_key):
        self.assertIsNotNone(_client_for_api_key("minimax-key"))
        client_for_api_key.assert_called_once_with("minimax-key", base_url="https://api.minimax.io/anthropic")
        getenv.assert_called_with("MINIMAX_API_BASE_URL")

    @patch("app.services.minimax_service._transport_async_client_for_api_key", return_value=object())
    @patch("app.services.minimax_service.os.getenv", return_value="https://api.minimax.io/anthropic")
    def test_async_client_uses_minimax_anthropic_base_url(self, getenv, async_client_for_api_key):
        self.assertIsNotNone(_async_client_for_api_key("minimax-key"))
        async_client_for_api_key.assert_called_once_with("minimax-key", base_url="https://api.minimax.io/anthropic")
        getenv.assert_called_with("MINIMAX_API_BASE_URL")

    @patch("app.services.minimax_service._anthropic_call_with_model_fallback", return_value=(None, None, []))
    @patch("app.services.minimax_service.os.getenv", return_value="https://api.minimax.io/anthropic")
    def test_transport_uses_minimax_anthropic_base_url(self, getenv, transport):
        _call_with_model_fallback("prompt", "MiniMax-M2.7", "MiniMax-M2.5", api_key="minimax-key")
        transport.assert_called_once()
        self.assertEqual(transport.call_args.kwargs.get("base_url"), "https://api.minimax.io/anthropic")
        getenv.assert_called_with("MINIMAX_API_BASE_URL")

    @patch("app.services.minimax_service._anthropic_call_with_model_fallback_async", return_value=(None, None, []))
    @patch("app.services.minimax_service.os.getenv", return_value="https://api.minimax.io/anthropic")
    def test_async_transport_uses_minimax_anthropic_base_url(self, getenv, transport):
        import asyncio

        asyncio.run(_call_with_model_fallback_async("prompt", "MiniMax-M2.7", "MiniMax-M2.5", api_key="minimax-key"))
        transport.assert_called_once()
        self.assertEqual(transport.call_args.kwargs.get("base_url"), "https://api.minimax.io/anthropic")
        getenv.assert_called_with("MINIMAX_API_BASE_URL")

    def test_llm_meta_preserves_minimax_provider_identity(self):
        llm = _llm_meta(
            None,
            "summary",
            "MiniMax-M2.5",
        )

        self.assertEqual(llm.get("provider"), "minimax")
        self.assertEqual(llm.get("model"), "MiniMax-M2.5")

    @patch("app.services.minimax_service._llm_meta", return_value={"provider": "minimax", "model": "MiniMax-M2.5"})
    @patch("app.services.minimax_service._message_text")
    @patch("app.services.minimax_service._call_with_model_fallback")
    @patch("app.services.minimax_service._client_for_api_key", return_value=object())
    def test_summarize_keeps_taxonomy_genre_from_structured_output(self, _client_for_api_key, call_with_model_fallback, message_text, _llm_meta):
        call_with_model_fallback.return_value = (object(), "MiniMax-M2.5", [])
        message_text.return_value = '{"summary":"要約です。","topics":["AI"],"genre":"research","other_label":"不要","translated_title":"翻訳済みタイトル","score_breakdown":{"importance":0.8,"novelty":0.5,"actionability":0.6,"reliability":0.9,"relevance":0.7},"score_reason":"理由です。"}'

        result = summarize(
            title="Example title",
            facts=["Fact 1"],
            source_text_chars=1200,
            model="MiniMax-M2.5",
            api_key="minimax-key",
        )

        self.assertEqual(result["genre"], "research")
        self.assertEqual(result["other_label"], "")


if __name__ == "__main__":
    unittest.main()
