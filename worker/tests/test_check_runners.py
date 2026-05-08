import unittest

from app.services.facts_check_runner import run_facts_check
from app.services.summary_faithfulness_runner import run_summary_faithfulness_check


class CheckRunnerShortCommentTests(unittest.TestCase):
    def test_facts_check_raises_after_retry_without_short_comment(self):
        with self.assertRaisesRegex(RuntimeError, "facts check short_comment missing"):
            run_facts_check(
                lambda: ("pass", {"provider": "test"}),
                retry_call=lambda: ("warn", {"provider": "retry"}),
                retry_attempts=1,
            )

    def test_summary_faithfulness_raises_after_retry_without_short_comment(self):
        with self.assertRaisesRegex(RuntimeError, "faithfulness short_comment missing"):
            run_summary_faithfulness_check(
                lambda: ("pass", {"provider": "test"}),
                retry_call=lambda: ("warn", {"provider": "retry"}),
                retry_attempts=1,
            )

    def test_facts_check_uses_retry_result_with_short_comment(self):
        result = run_facts_check(
            lambda: ("pass", {"provider": "test"}),
            retry_call=lambda: (
                '{"verdict":"warn","short_comment":"本文根拠が弱い抽出が含まれます。"}',
                {"provider": "retry"},
            ),
            retry_attempts=1,
        )

        self.assertEqual(result["verdict"], "warn")
        self.assertEqual(result["short_comment"], "本文根拠が弱い抽出が含まれます。")
        self.assertEqual(result["llm"], {"provider": "retry"})

    def test_summary_faithfulness_uses_retry_result_with_short_comment(self):
        result = run_summary_faithfulness_check(
            lambda: ("pass", {"provider": "test"}),
            retry_call=lambda: (
                '{"verdict":"warn","short_comment":"factsにない補足が一部含まれます。"}',
                {"provider": "retry"},
            ),
            retry_attempts=1,
        )

        self.assertEqual(result["verdict"], "warn")
        self.assertEqual(result["short_comment"], "factsにない補足が一部含まれます。")
        self.assertEqual(result["llm"], {"provider": "retry"})

    def test_check_runners_accept_json_with_short_comment(self):
        result = run_facts_check(
            lambda: ('{"verdict":"pass","short_comment":"本文で裏付けられています。"}', {"provider": "test"}),
            retry_call=lambda: ("fail", {"provider": "retry"}),
            retry_attempts=1,
        )

        self.assertEqual(result["verdict"], "pass")
        self.assertEqual(result["short_comment"], "本文で裏付けられています。")
        self.assertEqual(result["llm"], {"provider": "test"})

    def test_check_runners_accept_response_missing_opening_brace(self):
        result = run_facts_check(
            lambda: ('"verdict": "pass", "short_comment": "本文の内容を正確に抽出しています。" }', {"provider": "test"}),
            retry_call=lambda: ("fail", {"provider": "retry"}),
            retry_attempts=1,
        )

        self.assertEqual(result["verdict"], "pass")
        self.assertEqual(result["short_comment"], "本文の内容を正確に抽出しています。")
        self.assertEqual(result["llm"], {"provider": "test"})


if __name__ == "__main__":
    unittest.main()
