"""Messages transport fixtures only; no real API request or credentials."""
import copy
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

from maintenance import cli, report
from maintenance.plan import build_plan
from test_controller import ready


class AdviceTests(unittest.TestCase):
    def setUp(self):
        self.plan = build_plan(ready(), "a" * 40, now=1000)
        self.advice = {"summary_ja": "固定バージョンの変更を確認してください。", "warnings_ja": ["停止が発生する可能性があります。"]}
        self.text = {"type": "text", "text": json.dumps(self.advice)}

    def request(self, content=None, *, stop="end_turn", model=None, raw=None):
        response = io.BytesIO(raw if raw is not None else json.dumps({"stop_reason": stop, "content": content if content is not None else [self.text]}).encode())
        before = copy.deepcopy(self.plan)
        with patch.object(report.urllib.request, "urlopen", return_value=response) as transport:
            result = report.ai_advice(self.plan, "synthetic-key", **({"model": model} if model is not None else {}))
        self.assertEqual(self.plan, before)
        self.assertEqual(transport.call_args.kwargs, {"timeout": 30})
        body = json.loads(transport.call_args.args[0].data)
        return result, body

    def test_default_haiku_55_request_has_low_effort_and_bounded_output_budget(self):
        result, body = self.request()
        self.assertEqual(result, {"status": "available", **self.advice})
        self.assertEqual(body["model"], "claude-haiku-5-5")
        self.assertEqual(body["output_config"], {"effort": "low"})
        self.assertEqual(body["thinking"], {"type": "adaptive"})
        self.assertEqual(body["max_tokens"], 4096)
        self.assertFalse({"temperature", "top_p", "top_k"} & set(body))
        self.assertEqual(body["messages"][-1]["role"], "user")

    def test_thinking_and_redacted_blocks_are_ignored_without_leaking_them(self):
        thinking = {"type": "thinking", "thinking": "", "signature": "opaque-private-signature"}
        redacted = {"type": "redacted_thinking", "data": "opaque-private-data"}
        for blocks in ([thinking, self.text], [redacted, thinking, self.text], [self.text, thinking]):
            result, _ = self.request(blocks)
            self.assertEqual(result, {"status": "available", **self.advice})
            self.assertNotIn("opaque-private", json.dumps(result))

    def test_legacy_model_override_omits_haiku_55_only_parameters(self):
        for model in ("claude-haiku-4-5", "claude-haiku-4-5-20251001"):
            result, body = self.request(model=model)
            self.assertEqual(result["status"], "available")
            self.assertEqual(body["model"], model)
            self.assertEqual(body["max_tokens"], 1600)
            self.assertNotIn("output_config", body)
            self.assertNotIn("thinking", body)

    def test_refusal_tool_use_and_truncated_turns_are_unavailable(self):
        for stop in ("refusal", "tool_use", "max_tokens", "model_context_window_exceeded", None):
            result, _ = self.request(stop=stop)
            self.assertEqual(result["status"], "unavailable")

    def test_malformed_or_ambiguous_blocks_are_unavailable_even_with_valid_text(self):
        for content in ([], [self.text, self.text], [self.text, {"type": "tool_use", "name": "shell", "input": {}}],
                        [self.text, {"type": "thinking", "thinking": 123, "signature": "secret"}],
                        [self.text, {"type": "redacted_thinking", "data": []}], [self.text, None],
                        [self.text, {"type": "unknown", "text": "secret"}],
                        [{"type": "thinking", "thinking": "", "signature": "opaque"}]*9 + [self.text]):
            with self.subTest(content=content):
                result, _ = self.request(content)
                self.assertEqual(result["status"], "unavailable")
                self.assertNotIn("secret", json.dumps(result))

    def test_bad_json_schema_duplicate_fields_and_oversized_response_are_unavailable(self):
        for raw in (b'{"stop_reason":"end_turn","content":',
                    json.dumps({"stop_reason": "end_turn", "content": [{"type": "text", "text": '{"summary_ja":"x","summary_ja":"y","warnings_ja":[]}'}]}).encode(),
                    json.dumps({"stop_reason": "end_turn", "content": [{"type": "text", "text": '{"summary_ja":"x","warnings_ja":[],"commands":["id"]}'}]}).encode(),
                    b" " * (report.MAX_AI + 1)):
            result, _ = self.request(raw=raw)
            self.assertEqual(result["status"], "unavailable")

    def test_controller_default_and_explicit_override_use_same_model_contract(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "secrets").mkdir()
            (root / "secrets/hosts.json").write_text(json.dumps([{"host_id": "sifto-app", "address": "example.invalid", "port": 22, "role": "application"}]))
            (root / "secrets/anthropic_key").write_text("synthetic-key")
            git = Mock()
            git.head.return_value = "b" * 40
            git.read_at.return_value = None
            git.write.return_value = "c" * 40
            def local_path(value):
                return root / value.lstrip("/") if str(value).startswith(("/private", "/secrets")) else Path(value)
            history = {"host_id": "sifto-app", "receipts": [], "receipt_total": 0, "receipt_truncated": 0,
                       "receipt_history_complete": True, "unresolved_execution_count": 0}
            for configured, expected in [("", "claude-haiku-5-5"), ("claude-haiku-4-5", "claude-haiku-4-5")]:
                transport = Mock()
                transport.call.side_effect = [ready(), history]
                with patch.object(cli, "Path", side_effect=local_path), patch.object(cli, "Client", return_value=transport), \
                     patch.object(cli, "summary"), patch.object(cli, "github"), patch.object(cli, "output"), \
                     patch.dict("os.environ", {"MAINTENANCE_AI_MODEL": configured}), \
                     patch.object(cli, "ai_advice", return_value={"status": "available", **self.advice}) as ai:
                    cli.scan("a" * 40, "enjoydarts/sifto-operations", {"force_request": True, "dry_run": True}, git)
                self.assertEqual(ai.call_args.args[2], expected)
