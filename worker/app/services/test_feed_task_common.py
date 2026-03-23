import unittest

from app.services.feed_task_common import build_briefing_navigator_task


class FeedTaskCommonTests(unittest.TestCase):
    def test_build_briefing_navigator_task_includes_intro_structure_rules(self):
        task = build_briefing_navigator_task(
            persona="editor",
            candidates=[
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "summary": "Summary text",
                }
            ],
            intro_context={
                "now_jst": "2026-03-23T19:30:00+09:00",
                "date_jst": "2026-03-23",
                "weekday_jst": "Monday",
                "time_of_day": "evening",
                "season_hint": "early_spring",
            },
        )

        prompt = task["prompt"]
        self.assertIn("2〜3文", prompt)
        self.assertIn("時間帯", prompt)
        self.assertIn("季節", prompt)
        self.assertIn("不確かな記念日を断定しない", prompt)
        self.assertIn("橋渡し", prompt)

    def test_build_briefing_navigator_task_keeps_snark_safety_rules(self):
        task = build_briefing_navigator_task(
            persona="snark",
            candidates=[
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "summary": "Summary text",
                }
            ],
            intro_context={
                "now_jst": "2026-03-23T19:30:00+09:00",
                "date_jst": "2026-03-23",
                "weekday_jst": "Monday",
                "time_of_day": "evening",
                "season_hint": "early_spring",
            },
        )

        prompt = task["prompt"]
        self.assertIn("軽口", prompt)
        self.assertIn("不快・攻撃的・見下し表現は禁止", prompt)


if __name__ == "__main__":
    unittest.main()
