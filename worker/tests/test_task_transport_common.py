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

    def test_openrouter_llm_meta_tracks_requested_and_resolved_model(self):
        llm = openrouter_llm_meta(
            "openrouter::auto",
            "summary",
            {
                "input_tokens": 10,
                "output_tokens": 20,
                "cache_creation_input_tokens": 0,
                "cache_read_input_tokens": 0,
                "requested_model": "openrouter::auto",
                "resolved_model": "openai/gpt-oss-120b",
            },
        )

        self.assertEqual(llm.get("model"), "openrouter::auto")
        self.assertEqual(llm.get("requested_model"), "openrouter::auto")
        self.assertEqual(llm.get("resolved_model"), "openai/gpt-oss-120b")


if __name__ == "__main__":
    unittest.main()
