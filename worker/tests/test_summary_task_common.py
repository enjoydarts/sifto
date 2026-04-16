import unittest

from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import bind_prompt_override
from app.services.summary_task_common import SUMMARY_SYSTEM_INSTRUCTION, SUMMARY_TAXONOMY, build_summary_task


class SummaryTaskCommonTests(unittest.TestCase):
    def test_system_instruction_discourages_fact_by_fact_rewrites(self):
        self.assertIn("関連する facts を統合", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("「〜である。」の連続を避け", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("ニュースレター編集者", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("短文の羅列", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("技術記事は、一般的なビジネス記事より importance を明確に高く採点", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("actionability も高く採点", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("genre は必須です", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("固定 taxonomy", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("other_label", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("短い日本語", SUMMARY_SYSTEM_INSTRUCTION)
        for key in SUMMARY_TAXONOMY:
            self.assertIn(key, SUMMARY_SYSTEM_INSTRUCTION)

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
        self.assertIn("技術記事を明確に優遇してください", prompt)
        self.assertIn("importance と actionability を高めに採点", prompt)
        self.assertIn("genre は必須です", prompt)
        self.assertIn("other_label", prompt)
        self.assertIn("固定 taxonomy", prompt)
        self.assertEqual(task["schema"]["required"], ["summary", "topics", "translated_title", "score_breakdown", "score_reason", "genre", "other_label"])
        genre_schema = task["schema"]["properties"]["genre"]
        self.assertEqual(genre_schema["enum"], SUMMARY_TAXONOMY)
        self.assertEqual(task["schema"]["properties"]["other_label"]["type"], "string")
        self.assertEqual(task["schema"]["properties"]["other_label"]["maxLength"], 20)

    def test_genre_schema_allows_only_taxonomy_keys(self):
        task = build_summary_task(
            "OpenAI updates model lineup",
            ["OpenAI announced a new lineup."],
            source_text_chars=1200,
        )

        genre_schema = task["schema"]["properties"]["genre"]
        self.assertEqual(genre_schema["enum"], SUMMARY_TAXONOMY)
        self.assertEqual(len(set(genre_schema["enum"])), len(SUMMARY_TAXONOMY))

    def test_genre_schema_rejects_freeform_examples(self):
        task = build_summary_task(
            "OpenAI updates model lineup",
            ["OpenAI announced a new lineup."],
            source_text_chars=1200,
        )

        self.assertNotIn("技術", task["schema"]["properties"]["genre"]["enum"])
        self.assertNotIn("news", task["schema"]["properties"]["genre"]["enum"])
        self.assertNotIn("AI", task["schema"]["properties"]["genre"]["enum"])

    def test_taxonomy_no_longer_exposes_agent_key(self):
        self.assertIn("ai", SUMMARY_TAXONOMY)
        self.assertNotIn("agent", SUMMARY_TAXONOMY)
        self.assertIn("LLM", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("生成AI", SUMMARY_SYSTEM_INSTRUCTION)
        self.assertIn("エージェント", SUMMARY_SYSTEM_INSTRUCTION)

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
