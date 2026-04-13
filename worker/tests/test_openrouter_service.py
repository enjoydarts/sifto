import unittest
from unittest.mock import patch

from app.services.openrouter_service import _chat_json, _llm_meta, _repair_structured_json_text


class RepairStructuredJsonTextTests(unittest.TestCase):
    def test_repairs_malformed_json_for_openrouter_structured_output(self):
        with patch("app.services.openrouter_service._load_repair_json", return_value=lambda text, **kwargs: '{"summary":"ok"}'):
            got = _repair_structured_json_text(
                '{"summary":"ok"',
                "stepfun/step-3.5-flash:free",
                {"type": "object"},
            )

        self.assertEqual(got, '{"summary":"ok"}')

    def test_skips_repair_when_schema_is_not_requested(self):
        with patch("app.services.openrouter_service._load_repair_json", return_value=lambda text, **kwargs: '{"summary":"fixed"}'):
            got = _repair_structured_json_text(
                '{"summary":"ok"',
                "openai/gpt-oss-20b",
                None,
            )

        self.assertEqual(got, '{"summary":"ok"')

    def test_keeps_valid_json_without_repair(self):
        with patch("app.services.openrouter_service._load_repair_json") as mocked_loader:
            got = _repair_structured_json_text(
                '{"summary":"ok"}',
                "stepfun/step-3.5-flash:free",
                {"type": "object"},
            )

        self.assertEqual(got, '{"summary":"ok"}')
        mocked_loader.assert_not_called()


class OpenRouterLLMMetaTests(unittest.TestCase):
    def test_llm_meta_prefers_billed_cost_over_local_pricing(self):
        llm = _llm_meta(
            "openrouter::auto",
            "summary",
            {
                "input_tokens": 10,
                "output_tokens": 20,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "requested_model": "openrouter::auto",
                "resolved_model": "anthropic/claude-4.6-opus-20260205",
                "billed_cost_usd": 0.1234,
            },
        )

        self.assertEqual(llm.get("estimated_cost_usd"), 0.1234)

    def test_llm_meta_normalizes_dated_anthropic_resolved_model_to_canonical_family(self):
        llm = _llm_meta(
            "openrouter::auto",
            "summary",
            {
                "input_tokens": 10,
                "output_tokens": 20,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "requested_model": "openrouter::auto",
                "resolved_model": "anthropic/claude-4.6-opus-20260205",
            },
        )

        self.assertEqual(llm.get("pricing_model_family"), "anthropic/claude-opus-4.6")


class OpenRouterChatJsonTests(unittest.TestCase):
    def test_chat_json_fetches_generation_cost_when_usage_cost_is_missing(self):
        with patch(
            "app.services.openrouter_service.run_chat_json",
            return_value=(
                '{"answer":"ok"}',
                {
                    "input_tokens": 10,
                    "output_tokens": 20,
                    "cache_creation_input_tokens": 0,
                    "cache_read_input_tokens": 0,
                    "resolved_model": "anthropic/claude-4.6-opus-20260205",
                    "generation_id": "gen-123",
                },
            ),
        ), patch(
            "app.services.openrouter_service._fetch_generation_cost_details",
            return_value={"billed_cost_usd": 0.4321},
        ) as mocked_fetch:
            _text, usage = _chat_json("Return JSON", "openrouter::auto", "test-key")

        self.assertEqual(usage.get("billed_cost_usd"), 0.4321)
        mocked_fetch.assert_called_once_with("test-key", "gen-123", 90.0)

    def test_chat_json_skips_generation_lookup_when_billed_cost_already_exists(self):
        with patch(
            "app.services.openrouter_service.run_chat_json",
            return_value=(
                '{"answer":"ok"}',
                {
                    "input_tokens": 10,
                    "output_tokens": 20,
                    "cache_creation_input_tokens": 0,
                    "cache_read_input_tokens": 0,
                    "resolved_model": "anthropic/claude-4.6-opus-20260205",
                    "generation_id": "gen-123",
                    "billed_cost_usd": 0.1234,
                },
            ),
        ), patch(
            "app.services.openrouter_service._fetch_generation_cost_details",
        ) as mocked_fetch:
            _text, usage = _chat_json("Return JSON", "openrouter::auto", "test-key")

        self.assertEqual(usage.get("billed_cost_usd"), 0.1234)
        mocked_fetch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
