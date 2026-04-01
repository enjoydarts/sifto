import unittest

from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.summary_task_common import SUMMARY_SYSTEM_INSTRUCTION, build_summary_task


class SummaryTaskCommonTests(unittest.TestCase):
    def test_system_instruction_discourages_fact_by_fact_rewrites(self):
        self.assertIn("関連する facts を統合", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("「〜である。」の連続を避け", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("ニュースレター編集者", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("短文の羅列", SUMMARY_SYSTEM_INSTRUCTION)

    def test_build_summary_task_fallback_requests_natural_connected_prose(self):
        task = build_summary_task(
            "OpenAI updates model lineup",
            [
                "OpenAI announced a new lineup.",
                "The company said the release targets enterprise use.",
                "Pricing will change next month.",
            ],
            source_text_chars=2400,
        )

        prompt = task["prompt"]
        self.assertIn("各 fact を1文ずつ順番に言い換えるのではなく", prompt)
        self.assertIn("同じ文末表現を3文以上連続させない", prompt)
        self.assertIn("1段落目で記事の要点をまとめ", prompt)
        self.assertIn("ニュースレターの編集者が書く前文のように", prompt)
        self.assertIn("短文を切って並べるのではなく", prompt)
        self.assertIn("必要に応じて主語や関係を補い", prompt)

    def test_build_summary_task_fallback_uses_safe_translated_title_example(self):
        task = build_summary_task(
            "OpenAI updates model lineup",
            ["OpenAI announced a new lineup."],
            source_text_chars=1200,
        )

        prompt = task["prompt"]
        self.assertIn('"translated_title": ""', prompt)
        self.assertNotIn("英語タイトルの場合のみ日本語訳（日本語記事は空文字）", prompt)

    def test_default_template_override_matches_code_default_rendering(self):
        expected = build_summary_task(
            "OpenAI updates model lineup",
            [
                "OpenAI announced a new lineup.",
                "The company said the release targets enterprise use.",
                "Pricing will change next month.",
            ],
            source_text_chars=2400,
        )
        default_template = get_default_prompt_template("summary.default")

        with bind_prompt_override("summary.default", default_template["prompt_text"], default_template["system_instruction"]):
            actual = build_summary_task(
                "OpenAI updates model lineup",
                [
                    "OpenAI announced a new lineup.",
                    "The company said the release targets enterprise use.",
                    "Pricing will change next month.",
                ],
                source_text_chars=2400,
            )

        self.assertEqual(actual["system_instruction"], expected["system_instruction"])
        self.assertEqual(actual["prompt"], expected["prompt"])


if __name__ == "__main__":
    unittest.main()
