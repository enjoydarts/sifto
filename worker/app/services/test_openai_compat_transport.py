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


class _EmptyChoicesClient:
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
                "choices": [],
                "error": {"message": "upstream overloaded"},
                "provider": "openrouter",
            },
        )


class _EmptyContentClient:
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
                "choices": [
                    {
                        "finish_reason": "stop",
                        "message": {
                            "content": "",
                            "tool_calls": [{"type": "function", "function": {"name": "emit_json"}}],
                            "refusal": "",
                            "reasoning": {"tokens": 12},
                        },
                    }
                ],
                "provider": "openrouter",
                "usage": {"prompt_tokens": 1, "completion_tokens": 0},
            },
        )


class _ListLogger:
    def __init__(self):
        self.messages = []

    def warning(self, msg, *args):
        if args:
            msg = msg % args
        self.messages.append(msg)


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

    @patch("app.services.openai_compat_transport.httpx.Client", _EmptyChoicesClient)
    def test_empty_choices_error_includes_response_snippet(self):
        with self.assertRaises(RuntimeError) as ctx:
            run_chat_json(
                "Return JSON",
                "openai/gpt-oss-20b",
                "test-key",
                url="https://example.com/chat/completions",
                normalize_model_name=lambda model: model,
                supports_strict_schema=lambda model: False,
                timeout_sec=5,
                attempts=1,
                base_sleep_sec=0,
                provider_name="openrouter",
                logger=None,
                response_schema={"type": "object"},
            )

        self.assertIn("empty choices", str(ctx.exception))
        self.assertIn("upstream overloaded", str(ctx.exception))

    @patch("app.services.openai_compat_transport.httpx.Client", _EmptyContentClient)
    def test_empty_content_logs_message_shape(self):
        logger = _ListLogger()

        text, usage = run_chat_json(
            "Return JSON",
            "openai/gpt-oss-20b",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="openrouter",
            logger=logger,
            response_schema={"type": "object"},
        )

        self.assertEqual(text, "")
        self.assertEqual(usage["input_tokens"], 1)
        self.assertTrue(logger.messages)
        self.assertIn("empty message content", logger.messages[0])
        self.assertIn("tool_calls_count=1", logger.messages[0])
        self.assertIn("message_keys=['content', 'reasoning', 'refusal', 'tool_calls']", logger.messages[0])


if __name__ == "__main__":
    unittest.main()
