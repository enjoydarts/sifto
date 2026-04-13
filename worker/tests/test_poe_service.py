import unittest
from unittest.mock import patch

import httpx

from app.services.llm_catalog import provider_for_model, resolve_model_id
from app.services.poe_service import _chat_json, _llm_meta, _supports_anthropic_transport


class _PoeAnthropicClient:
    def __init__(self, *args, **kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def post(self, url, headers=None, json=None):
        return httpx.Response(
            200,
            json={
                "id": "msg_123",
                "model": "claude-sonnet-4.5",
                "content": [{"type": "text", "text": '{"answer":"ok"}'}],
                "usage": {"input_tokens": 11, "output_tokens": 7},
            },
        )


class PoeServiceTests(unittest.TestCase):
    def test_resolve_model_id_strips_poe_alias(self):
        self.assertEqual(resolve_model_id("poe::Claude-Sonnet-4.5"), "Claude-Sonnet-4.5")

    def test_provider_for_model_detects_poe_alias(self):
        self.assertEqual(provider_for_model("poe::Claude-Sonnet-4.5"), "poe")

    def test_supports_anthropic_transport_for_claude_models(self):
        self.assertTrue(_supports_anthropic_transport("poe::Claude-Sonnet-4.5"))
        self.assertFalse(_supports_anthropic_transport("poe::GPT-5-Codex"))

    @patch("app.services.poe_service.httpx.Client", _PoeAnthropicClient)
    def test_chat_json_uses_anthropic_compat_for_claude_models(self):
        text, usage = _chat_json(
            "Return JSON",
            "poe::Claude-Sonnet-4.5",
            "test-key",
            response_schema={"type": "object"},
        )

        self.assertEqual(text, '{"answer":"ok"}')
        self.assertEqual(usage.get("requested_model"), "poe::Claude-Sonnet-4.5")
        self.assertEqual(usage.get("resolved_model"), "claude-sonnet-4.5")

    def test_llm_meta_preserves_requested_and_resolved_model(self):
        llm = _llm_meta(
            "poe::Claude-Sonnet-4.5",
            "summary",
            {
                "input_tokens": 10,
                "output_tokens": 20,
                "requested_model": "poe::Claude-Sonnet-4.5",
                "resolved_model": "claude-sonnet-4.5",
            },
        )

        self.assertEqual(llm.get("provider"), "poe")
        self.assertEqual(llm.get("model"), "poe::Claude-Sonnet-4.5")
        self.assertEqual(llm.get("requested_model"), "poe::Claude-Sonnet-4.5")
        self.assertEqual(llm.get("resolved_model"), "claude-sonnet-4.5")


if __name__ == "__main__":
    unittest.main()
