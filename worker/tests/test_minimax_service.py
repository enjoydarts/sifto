import unittest
from unittest.mock import patch

from app.services.llm_catalog import provider_api_key_header, provider_for_model
from app.services.minimax_service import _chat_completions_url, _llm_meta, _p


class MiniMaxServiceTests(unittest.TestCase):
    def test_provider_for_model_detects_minimax(self):
        self.assertEqual(provider_for_model("MiniMax-M2.5"), "minimax")
        self.assertEqual(provider_for_model("MiniMax-M2.7"), "minimax")
        self.assertEqual(provider_for_model("minimax::MiniMax-M2.7"), "minimax")
        self.assertEqual(provider_for_model("minimax/MiniMax-M2.7"), "minimax")

    def test_provider_api_key_header_uses_minimax_header(self):
        self.assertEqual(provider_api_key_header("minimax"), "x-minimax-api-key")

    def test_chat_completions_url_normalizes_base_url(self):
        self.assertEqual(_chat_completions_url(""), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/v1"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io"), "https://api.minimax.io/v1/chat/completions")
        self.assertEqual(_chat_completions_url("https://api.minimax.io/chat/completions"), "https://api.minimax.io/chat/completions")

    @patch("app.services.minimax_service.os.getenv", return_value="https://api.minimax.io/v1")
    def test_provider_uses_normalized_env_chat_url(self, getenv):
        self.assertEqual(_p._get_chat_url(), "https://api.minimax.io/v1/chat/completions")
        getenv.assert_called()

    def test_llm_meta_preserves_minimax_provider_identity(self):
        llm = _llm_meta(
            "MiniMax-M2.5",
            "summary",
            {
                "input_tokens": 120,
                "output_tokens": 80,
            },
        )

        self.assertEqual(llm.get("provider"), "minimax")
        self.assertEqual(llm.get("model"), "MiniMax-M2.5")


if __name__ == "__main__":
    unittest.main()
