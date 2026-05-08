import unittest

from app.services.facts_check_runner import run_facts_check
from app.services.summary_faithfulness_runner import run_summary_faithfulness_check


class CheckRunnerShortCommentTests(unittest.TestCase):
    def test_facts_check_requires_model_short_comment_even_for_line_verdict(self):
        result = run_facts_check(
            lambda: ("pass", {"provider": "test"}),
            retry_call=lambda: ("warn", {"provider": "retry"}),
            retry_attempts=1,
        )

        self.assertEqual(result["verdict"], "warn")
        self.assertEqual(
            result["short_comment"],
            "事実抽出チェックの判定応答を取得できなかったため未確認です。再試行してください。",
        )
        self.assertEqual(result["llm"], {"provider": "retry"})

    def test_summary_faithfulness_requires_model_short_comment_even_for_line_verdict(self):
        result = run_summary_faithfulness_check(
            lambda: ("pass", {"provider": "test"}),
            retry_call=lambda: ("warn", {"provider": "retry"}),
            retry_attempts=1,
        )

        self.assertEqual(result["verdict"], "warn")
        self.assertEqual(
            result["short_comment"],
            "忠実性チェックの判定応答を取得できなかったため未確認です。再試行してください。",
        )
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


if __name__ == "__main__":
    unittest.main()
