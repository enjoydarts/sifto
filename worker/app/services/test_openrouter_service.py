import unittest
from unittest.mock import patch

from app.services.openrouter_service import _llm_meta, _repair_structured_json_text


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


if __name__ == "__main__":
    unittest.main()
