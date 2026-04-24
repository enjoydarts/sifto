import unittest

from app.services.llm_text_utils import (
    audio_briefing_script_max_tokens,
    facts_need_japanese_localization,
    summary_max_tokens,
)


class FactsLocalizationDetectionTests(unittest.TestCase):
    def test_chinese_facts_still_need_japanese_localization(self):
        facts = [
            "苹果公司发布新模型，并计划扩大企业销售。",
            "该公司表示将在亚洲市场继续投资。",
        ]

        self.assertTrue(facts_need_japanese_localization(facts))

    def test_japanese_facts_with_kana_do_not_need_localization(self):
        facts = [
            "アップルは新モデルを発表し、企業向け販売を拡大すると述べた。",
            "同社はアジア市場への投資を続ける方針を示した。",
        ]

        self.assertFalse(facts_need_japanese_localization(facts))

    def test_audio_briefing_script_max_tokens_uses_doubled_target_chars_with_updated_cap(self):
        self.assertEqual(audio_briefing_script_max_tokens(14000), 28000)

    def test_audio_briefing_script_max_tokens_boosts_duo_mode(self):
        self.assertGreater(audio_briefing_script_max_tokens(600, "duo"), audio_briefing_script_max_tokens(600, "single"))
        self.assertEqual(audio_briefing_script_max_tokens(14000, "duo"), 60000)

    def test_summary_max_tokens_increases_by_about_fifteen_percent(self):
        self.assertEqual(summary_max_tokens(500), 1400)
        self.assertEqual(summary_max_tokens(600), 1400)

    def test_summary_max_tokens_keeps_existing_bounds(self):
        self.assertEqual(summary_max_tokens(100), 1400)
        self.assertEqual(summary_max_tokens(3000), 4140)
        self.assertEqual(summary_max_tokens(4000), 5200)


if __name__ == "__main__":
    unittest.main()
