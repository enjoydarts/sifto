import unittest
from unittest.mock import patch

import httpx

from app.services.openai_compat_transport import run_chat_json


class _FakeClient:
    last_json = None

    def __init__(self, *args, **kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def post(self, url, headers=None, json=None):
        _FakeClient.last_json = json
        return httpx.Response(
            200,
            json={
                "choices": [{"message": {"content": '{"answer":"ok"}'}}],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1},
            },
        )


class RunChatJsonTests(unittest.TestCase):
    def setUp(self):
        _FakeClient.last_json = None

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_zai_requests_disable_thinking(self):
        run_chat_json(
            "Return JSON",
            "glm-5",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="zai",
            logger=None,
            response_schema={"type": "object"},
        )

        self.assertIsNotNone(_FakeClient.last_json)
        self.assertEqual(_FakeClient.last_json.get("thinking"), {"type": "disabled"})

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_non_zai_requests_do_not_set_thinking(self):
        run_chat_json(
            "Return JSON",
            "gpt-5-mini",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="openai",
            logger=None,
            response_schema={"type": "object"},
        )

        self.assertIsNotNone(_FakeClient.last_json)
        self.assertNotIn("thinking", _FakeClient.last_json)


if __name__ == "__main__":
    unittest.main()
