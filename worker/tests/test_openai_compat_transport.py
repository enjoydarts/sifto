import unittest
import threading
import time
import os
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


class _ResolvedModelClient:
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
                "model": "openai/gpt-oss-120b",
                "choices": [{"message": {"content": '{"answer":"ok"}'}}],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1},
            },
        )


class _ResolvedModelWithCostClient:
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
                "id": "gen-123",
                "model": "anthropic/claude-4.6-opus-20260205",
                "choices": [{"message": {"content": '{"answer":"ok"}'}}],
                "usage": {
                    "prompt_tokens": 12,
                    "completion_tokens": 34,
                    "cost": 0.1234,
                },
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


class _RetryThenSuccessClient:
    call_count = 0

    def __init__(self, *args, **kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def post(self, url, headers=None, json=None):
        _RetryThenSuccessClient.call_count += 1
        if _RetryThenSuccessClient.call_count == 1:
            return httpx.Response(
                429,
                json={
                    "error": {"message": "Rate limit reached"},
                },
            )
        return httpx.Response(
            200,
            json={
                "model": "openai/gpt-oss-120b",
                "choices": [{"message": {"content": '{"answer":"ok"}'}}],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1},
            },
        )


class _EmptyLengthThenSuccessClient:
    call_count = 0

    def __init__(self, *args, **kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def post(self, url, headers=None, json=None):
        _EmptyLengthThenSuccessClient.call_count += 1
        if _EmptyLengthThenSuccessClient.call_count == 1:
            return httpx.Response(
                200,
                json={
                    "provider": "DeepInfra",
                    "choices": [
                        {
                            "finish_reason": "length",
                            "message": {
                                "content": None,
                                "refusal": "",
                                "reasoning": {"tokens": 123},
                            },
                        }
                    ],
                    "usage": {"prompt_tokens": 11, "completion_tokens": 6400},
                },
            )
        return httpx.Response(
            200,
            json={
                "model": "moonshotai/kimi-k2.5",
                "provider": "DeepInfra",
                "choices": [{"finish_reason": "stop", "message": {"content": '{"answer":"ok"}'}}],
                "usage": {"prompt_tokens": 12, "completion_tokens": 34},
            },
        )


class _ListLogger:
    def __init__(self):
        self.messages = []

    def warning(self, msg, *args):
        if args:
            msg = msg % args
        self.messages.append(msg)


class _ConcurrentTrackingClient:
    active_count = 0
    max_active_count = 0
    lock = threading.Lock()

    def __init__(self, *args, **kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def post(self, url, headers=None, json=None):
        with _ConcurrentTrackingClient.lock:
            _ConcurrentTrackingClient.active_count += 1
            if _ConcurrentTrackingClient.active_count > _ConcurrentTrackingClient.max_active_count:
                _ConcurrentTrackingClient.max_active_count = _ConcurrentTrackingClient.active_count
        time.sleep(0.05)
        with _ConcurrentTrackingClient.lock:
            _ConcurrentTrackingClient.active_count -= 1
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
        _RetryThenSuccessClient.call_count = 0
        _EmptyLengthThenSuccessClient.call_count = 0
        _ConcurrentTrackingClient.active_count = 0
        _ConcurrentTrackingClient.max_active_count = 0

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
    def test_moonshot_requests_disable_thinking(self):
        run_chat_json(
            "Return JSON",
            "kimi-k2.5",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="moonshot",
            logger=None,
            response_schema={"type": "object"},
        )

        self.assertIsNotNone(_FakeClient.last_json)
        self.assertEqual(_FakeClient.last_json.get("thinking"), {"type": "disabled"})

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_featherless_requests_disable_qwen_thinking_via_chat_template_kwargs(self):
        run_chat_json(
            "Return JSON",
            "Qwen/Qwen3.5-27B",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="featherless",
            logger=None,
            response_schema={"type": "object"},
        )

        self.assertIsNotNone(_FakeClient.last_json)
        self.assertEqual(
            _FakeClient.last_json.get("chat_template_kwargs"),
            {"enable_thinking": False},
        )
        self.assertNotIn("reasoning", _FakeClient.last_json)
        self.assertNotIn("thinking", _FakeClient.last_json)

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_featherless_requests_disable_kimi_k25_thinking_via_top_level_thinking(self):
        run_chat_json(
            "Return JSON",
            "moonshotai/Kimi-K2.5",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="featherless",
            logger=None,
            response_schema={"type": "object"},
        )

        self.assertIsNotNone(_FakeClient.last_json)
        self.assertEqual(_FakeClient.last_json.get("thinking"), {"type": "disabled"})
        self.assertEqual(
            _FakeClient.last_json.get("chat_template_kwargs"),
            {"enable_thinking": False},
        )

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_featherless_requests_disable_glm_thinking_like_native_zai(self):
        run_chat_json(
            "Return JSON",
            "zai-org/GLM-5.1",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=1,
            base_sleep_sec=0,
            provider_name="featherless",
            logger=None,
            response_schema={"type": "object"},
        )

        self.assertIsNotNone(_FakeClient.last_json)
        self.assertEqual(_FakeClient.last_json.get("thinking"), {"type": "disabled"})
        self.assertNotIn("chat_template_kwargs", _FakeClient.last_json)

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_non_zai_requests_do_not_set_thinking(self):
        _text, usage = run_chat_json(
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
        self.assertEqual(usage.get("requested_model"), "gpt-5-mini")

    @patch("app.services.openai_compat_transport.httpx.Client", _FakeClient)
    def test_custom_temperature_and_top_p_override_defaults(self):
        run_chat_json(
            "Return JSON",
            "gpt-4.1-mini",
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
            temperature=0.7,
            top_p=0.98,
        )

        self.assertEqual(_FakeClient.last_json.get("temperature"), 0.7)
        self.assertEqual(_FakeClient.last_json.get("top_p"), 0.98)

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

    @patch("app.services.openai_compat_transport.httpx.Client", _ResolvedModelClient)
    def test_usage_includes_requested_and_resolved_model(self):
        _text, usage = run_chat_json(
            "Return JSON",
            "openrouter::auto",
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

        self.assertEqual(usage.get("requested_model"), "openrouter::auto")
        self.assertEqual(usage.get("resolved_model"), "openai/gpt-oss-120b")

    @patch("app.services.openai_compat_transport.httpx.Client", _ResolvedModelWithCostClient)
    def test_usage_includes_openrouter_billed_cost_and_generation_id(self):
        _text, usage = run_chat_json(
            "Return JSON",
            "openrouter::auto",
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

        self.assertEqual(usage.get("resolved_model"), "anthropic/claude-4.6-opus-20260205")
        self.assertEqual(usage.get("billed_cost_usd"), 0.1234)
        self.assertEqual(usage.get("generation_id"), "gen-123")

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

    @patch("app.services.openai_compat_transport.httpx.Client", _RetryThenSuccessClient)
    def test_retryable_status_is_recorded_as_execution_failure_when_later_success(self):
        _text, usage = run_chat_json(
            "Return JSON",
            "openrouter::auto",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=2,
            base_sleep_sec=0,
            provider_name="openrouter",
            logger=_ListLogger(),
            response_schema={"type": "object"},
        )

        self.assertEqual(
            usage.get("execution_failures"),
            [{"model": "openrouter::auto", "reason": "status=429 body={\"error\":{\"message\":\"Rate limit reached\"}}"}],
        )

    @patch("app.services.openai_compat_transport.httpx.Client", _EmptyLengthThenSuccessClient)
    def test_empty_json_content_with_length_retries_and_records_execution_failure(self):
        _text, usage = run_chat_json(
            "Return JSON",
            "openrouter::moonshotai/kimi-k2.5",
            "test-key",
            url="https://example.com/chat/completions",
            normalize_model_name=lambda model: model,
            supports_strict_schema=lambda model: False,
            timeout_sec=5,
            attempts=2,
            base_sleep_sec=0,
            provider_name="openrouter",
            logger=_ListLogger(),
            response_schema={"type": "object"},
        )

        self.assertEqual(_text, '{"answer":"ok"}')
        self.assertEqual(
            usage.get("execution_failures"),
            [{"model": "openrouter::moonshotai/kimi-k2.5", "reason": "empty_json_content finish_reason=length provider=DeepInfra"}],
        )
        self.assertEqual(usage.get("resolved_model"), "moonshotai/kimi-k2.5")

    @patch("app.services.openai_compat_transport.httpx.Client", _EmptyContentClient)
    def test_empty_json_content_with_stop_retries_and_records_execution_failure(self):
        class _EmptyStopThenSuccessClient:
            call_count = 0

            def __init__(self, *args, **kwargs):
                pass

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def post(self, url, headers=None, json=None):
                _EmptyStopThenSuccessClient.call_count += 1
                if _EmptyStopThenSuccessClient.call_count == 1:
                    return httpx.Response(
                        200,
                        json={
                            "provider": "DeepInfra",
                            "choices": [
                                {
                                    "finish_reason": "stop",
                                    "message": {
                                        "content": "",
                                        "refusal": "",
                                        "reasoning": {"tokens": 12},
                                    },
                                }
                            ],
                            "usage": {"prompt_tokens": 11, "completion_tokens": 0},
                        },
                    )
                return httpx.Response(
                    200,
                    json={
                        "model": "z-ai/glm-4.7-flash",
                        "provider": "DeepInfra",
                        "choices": [{"finish_reason": "stop", "message": {"content": '{"answer":"ok"}'}}],
                        "usage": {"prompt_tokens": 12, "completion_tokens": 34},
                    },
                )

        _EmptyStopThenSuccessClient.call_count = 0
        with patch("app.services.openai_compat_transport.httpx.Client", _EmptyStopThenSuccessClient):
            _text, usage = run_chat_json(
                "Return JSON",
                "openrouter::z-ai/glm-4.7-flash",
                "test-key",
                url="https://example.com/chat/completions",
                normalize_model_name=lambda model: model,
                supports_strict_schema=lambda model: False,
                timeout_sec=5,
                attempts=2,
                base_sleep_sec=0,
                provider_name="openrouter",
                logger=_ListLogger(),
                response_schema={"type": "object"},
            )

        self.assertEqual(_text, '{"answer":"ok"}')
        self.assertEqual(
            usage.get("execution_failures"),
            [{"model": "openrouter::z-ai/glm-4.7-flash", "reason": "empty_json_content finish_reason=stop provider=DeepInfra"}],
        )
        self.assertEqual(usage.get("resolved_model"), "z-ai/glm-4.7-flash")

    @patch("app.services.openai_compat_transport.httpx.Client", _ConcurrentTrackingClient)
    def test_zai_requests_are_serialized_by_default(self):
        previous = os.environ.get("ZAI_MAX_CONCURRENCY")
        os.environ["ZAI_MAX_CONCURRENCY"] = "1"
        try:
            threads = [
                threading.Thread(
                    target=run_chat_json,
                    kwargs={
                        "prompt": "Return JSON",
                        "model": "glm-5-turbo",
                        "api_key": "test-key",
                        "url": "https://example.com/chat/completions",
                        "normalize_model_name": lambda model: model,
                        "supports_strict_schema": lambda model: False,
                        "timeout_sec": 5,
                        "attempts": 1,
                        "base_sleep_sec": 0,
                        "provider_name": "zai",
                        "logger": _ListLogger(),
                        "response_schema": {"type": "object"},
                    },
                )
                for _ in range(2)
            ]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join()
        finally:
            if previous is None:
                os.environ.pop("ZAI_MAX_CONCURRENCY", None)
            else:
                os.environ["ZAI_MAX_CONCURRENCY"] = previous

        self.assertEqual(_ConcurrentTrackingClient.max_active_count, 1)


if __name__ == "__main__":
    unittest.main()
