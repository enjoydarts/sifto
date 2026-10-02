import asyncio
import json
import unittest
from contextlib import ExitStack
from unittest.mock import patch

import httpx

from app.services.xiaomi_mimo_token_plan_service import _p


SYNC_CLIENT = httpx.Client
ASYNC_CLIENT = httpx.AsyncClient
COMPLETE_SUMMARY = {
    "summary": "つくば駅前と大学を結ぶ自動運転バスの運行が始まりました。",
    "topics": [],
    "genre": "other",
    "other_label": "交通",
    "translated_title": "",
    "score_breakdown": {},
    "score_reason": "公共交通の運行開始に関する情報です。",
}


class MiMoSummaryTruncationTests(unittest.TestCase):
    def _summarize(self, responses, *, asynchronous=False, attempts=3, facts_task=False):
        self.requests = []

        def respond(request):
            self.requests.append(json.loads(request.content))
            payload = responses[min(len(self.requests) - 1, len(responses) - 1)]
            return httpx.Response(200, json=payload)

        transport = httpx.MockTransport(respond)
        with ExitStack() as stack:
            stack.enter_context(patch.dict("os.environ", {"XIAOMI_MIMO_TOKEN_PLAN_RETRY_ATTEMPTS": str(attempts)}))
            stack.enter_context(patch("app.services.openai_compat_transport.httpx.Client", side_effect=lambda **kwargs: SYNC_CLIENT(transport=transport, **kwargs)))
            stack.enter_context(patch("app.services.openai_compat_transport.httpx.AsyncClient", side_effect=lambda **kwargs: ASYNC_CLIENT(transport=transport, **kwargs)))
            stack.enter_context(patch("app.services.openai_compat_transport.time.sleep"))
            stack.enter_context(patch("app.services.openai_compat_transport.asyncio.sleep"))
            kwargs = dict(title="自動運転バスの運行開始", model="mimo-v2.6-flash", api_key="test-key")
            if facts_task:
                kwargs["content"] = "つくば駅前と大学を結ぶバスが運行を開始した。"
                call = _p.extract_facts_async if asynchronous else _p.extract_facts
            else:
                kwargs.update(facts=["つくば駅前と大学を結ぶバスが運行を開始した。"], source_text_chars=1577)
                call = _p.summarize_async if asynchronous else _p.summarize
            if asynchronous:
                return asyncio.run(call(**kwargs))
            return call(**kwargs)

    def _response(self, content, finish_reason, tokens=100):
        return {
            "choices": [{"finish_reason": finish_reason, "message": {"content": content}}],
            "usage": {"prompt_tokens": 2908, "completion_tokens": tokens, "completion_tokens_details": {"reasoning_tokens": 90}},
        }

    def test_summary_disables_thinking_in_sync_and_async_requests(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                result = self._summarize([self._response(json.dumps(COMPLETE_SUMMARY), "stop")], asynchronous=asynchronous)
                self.assertEqual(result["summary"], COMPLETE_SUMMARY["summary"])
                self.assertEqual(self.requests[0].get("thinking"), {"type": "disabled"})
                self.assertEqual(self.requests[0].get("max_completion_tokens"), 1400)
                self.assertNotIn("max_tokens", self.requests[0])

    def test_truncated_summary_retries_with_more_tokens_in_sync_and_async(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                result = self._summarize([
                    self._response('{"summary":"つくば駅前と大学を結ぶバスが運行を開始し、土日', "length", 1400),
                    self._response(json.dumps(COMPLETE_SUMMARY), "stop"),
                ], asynchronous=asynchronous)
                self.assertEqual(result["summary"], COMPLETE_SUMMARY["summary"])
                self.assertEqual([request["max_completion_tokens"] for request in self.requests], [1400, 2800])
                self.assertIn("finish_reason=length", result["llm"]["execution_failures"][0]["reason"])

    def test_repeated_truncation_raises_instead_of_returning_partial_summary(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                with self.assertRaisesRegex(RuntimeError, "finish_reason=length"):
                    self._summarize([self._response('{"summary":"途中で切れた本文', "length", 1400)], asynchronous=asynchronous, attempts=3)
                self.assertEqual([request["max_completion_tokens"] for request in self.requests], [1400, 2800, 5600])

    def test_length_response_is_rejected_even_if_json_is_valid(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                with self.assertRaisesRegex(RuntimeError, "finish_reason=length"):
                    self._summarize([self._response(json.dumps(COMPLETE_SUMMARY), "length", 1400)], asynchronous=asynchronous, attempts=1)

    def test_unclosed_summary_is_rejected_even_without_length_finish_reason(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                with self.assertRaisesRegex(RuntimeError, "summarize parse failed"):
                    self._summarize([self._response('{"summary":"途中で切れた本文', "stop")], asynchronous=asynchronous)

    def test_empty_length_response_retries_with_more_tokens(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                result = self._summarize([
                    self._response("", "length", 1400),
                    self._response(json.dumps(COMPLETE_SUMMARY), "stop"),
                ], asynchronous=asynchronous)
                self.assertEqual(result["summary"], COMPLETE_SUMMARY["summary"])
                self.assertEqual([request["max_completion_tokens"] for request in self.requests], [1400, 2800])

    def test_complete_summary_is_salvaged_when_later_metadata_is_malformed(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                result = self._summarize([self._response('{"summary":"完成した要約です。","topics":[', "stop")], asynchronous=asynchronous)
                self.assertEqual(result["summary"], "完成した要約です。")

    def test_thinking_override_does_not_apply_to_facts(self):
        for asynchronous in (False, True):
            with self.subTest(asynchronous=asynchronous):
                result = self._summarize([self._response('{"facts":["日本語の事実です。"]}', "stop")], asynchronous=asynchronous, facts_task=True)
                self.assertEqual(result["facts"], ["日本語の事実です。"])
                self.assertNotIn("thinking", self.requests[0])
                self.assertEqual(self.requests[0]["max_tokens"], 3000)
