import unittest
from unittest.mock import patch

from app.services.openrouter_service import _repair_structured_json_text


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


if __name__ == "__main__":
    unittest.main()
