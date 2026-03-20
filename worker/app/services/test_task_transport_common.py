import unittest

from app.services.openrouter_service import _llm_meta as openrouter_llm_meta
from app.services.task_transport_common import with_execution_failures


class WithExecutionFailuresTests(unittest.TestCase):
    def test_with_execution_failures_copies_valid_failures(self):
        llm = {"provider": "openrouter", "model": "openrouter::test/model"}
        out = with_execution_failures(
            llm,
            [
                {"model": "openrouter::primary/model", "reason": "rate limit"},
                {"model": "", "reason": "ignored"},
                "ignored",
            ],
        )

        self.assertEqual(
            out.get("execution_failures"),
            [{"model": "openrouter::primary/model", "reason": "rate limit"}],
        )

    def test_openrouter_llm_meta_preserves_execution_failures_from_usage(self):
        llm = openrouter_llm_meta(
            "openrouter::fallback/model",
            "summary",
            {
                "input_tokens": 10,
                "output_tokens": 20,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "execution_failures": [
                    {"model": "openrouter::primary/model", "reason": "rate limit"}
                ],
            },
        )

        self.assertEqual(
            llm.get("execution_failures"),
            [{"model": "openrouter::primary/model", "reason": "rate limit"}],
        )


if __name__ == "__main__":
    unittest.main()
