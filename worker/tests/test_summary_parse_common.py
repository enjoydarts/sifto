import unittest

from app.services.summary_parse_common import finalize_summary_result


class SummaryParseCommonTests(unittest.TestCase):
    def test_finalize_summary_result_sets_score_policy_version_v4(self):
        result = finalize_summary_result(
            title="Example",
            summary_text="要約です。",
            topics=["AI"],
            raw_score_breakdown={
                "importance": 0.8,
                "novelty": 0.5,
                "actionability": 0.6,
                "reliability": 0.9,
                "relevance": 0.7,
            },
            score_reason="理由です。",
            translated_title="",
            translate_func=lambda raw: raw,
            llm={"provider": "test", "model": "test"},
            error_prefix="test",
            response_text='{"summary":"要約です。"}',
        )

        self.assertEqual(result["score_policy_version"], "v4")


if __name__ == "__main__":
    unittest.main()
