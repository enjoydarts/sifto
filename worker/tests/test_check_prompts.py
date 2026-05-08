import unittest
from unittest.mock import patch

from app.services.facts_check_common import facts_check_prompt, facts_check_retry_prompt, facts_check_system_instruction
from app.services.summary_faithfulness_common import (
    SUMMARY_FAITHFULNESS_SCHEMA,
    summary_faithfulness_prompt,
    summary_faithfulness_retry_prompt,
    summary_faithfulness_system_instruction,
)


class CheckPromptShortCommentTests(unittest.TestCase):
    def test_facts_check_prompts_require_short_comment_json(self):
        texts = [
            facts_check_system_instruction(),
            facts_check_prompt("title", "本文です。", ["本文に基づく fact です。"]),
            facts_check_retry_prompt("title", "本文です。", ["本文に基づく fact です。"]),
        ]

        for text in texts:
            self.assertIn("short_comment", text)
            self.assertIn("JSON", text)
            self.assertIn("verdict だけ", text)

    def test_summary_faithfulness_prompts_require_short_comment_json(self):
        texts = [
            summary_faithfulness_system_instruction(),
            summary_faithfulness_prompt("title", ["fact です。"], "summary です。"),
            summary_faithfulness_retry_prompt("title", ["fact です。"], "summary です。"),
        ]

        for text in texts:
            self.assertIn("short_comment", text)
            self.assertIn("JSON", text)
            self.assertIn("verdict だけ", text)

    def test_summary_faithfulness_schema_requires_short_comment(self):
        self.assertIn("short_comment", SUMMARY_FAITHFULNESS_SCHEMA["required"])
        self.assertEqual(SUMMARY_FAITHFULNESS_SCHEMA["properties"]["short_comment"]["type"], "string")

    def test_facts_check_prompt_appends_contract_to_langfuse_override(self):
        with patch("app.services.facts_check_common.get_prompt_text", return_value="外部プロンプト"):
            text = facts_check_prompt("title", "本文です。", ["fact です。"])

        self.assertIn("外部プロンプト", text)
        self.assertIn("必須キーは verdict と short_comment の 2 つ", text)
        self.assertIn('"short_comment"', text)

    def test_summary_faithfulness_prompt_appends_contract_to_langfuse_override(self):
        with patch("app.services.summary_faithfulness_common.get_prompt_text", return_value="外部プロンプト"):
            text = summary_faithfulness_prompt("title", ["fact です。"], "summary です。")

        self.assertIn("外部プロンプト", text)
        self.assertIn("必須キーは verdict と short_comment の 2 つ", text)
        self.assertIn('"short_comment"', text)


if __name__ == "__main__":
    unittest.main()
