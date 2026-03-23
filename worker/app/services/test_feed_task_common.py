import unittest
from unittest.mock import patch

from app.services.feed_task_common import (
    ASK_NAVIGATOR_SCHEMA,
    BRIEFING_NAVIGATOR_SCHEMA,
    SOURCE_NAVIGATOR_SCHEMA,
    _resolve_persona_file,
    build_ask_navigator_task,
    build_briefing_navigator_task,
    build_source_navigator_task,
)


class FeedTaskCommonTests(unittest.TestCase):
    def test_resolve_persona_file_prefers_llm_catalog_dir(self):
        with patch.dict("os.environ", {"NAVIGATOR_PERSONAS_PATH": "", "LLM_CATALOG_PATH": "/app/shared/llm_catalog.json"}, clear=False):
            path = _resolve_persona_file()
        self.assertEqual(path.as_posix(), "/app/shared/ai_navigator_personas.json")

    def test_briefing_navigator_schema_requires_all_pick_fields_for_strict_json_schema(self):
        pick_schema = BRIEFING_NAVIGATOR_SCHEMA["properties"]["picks"]["items"]

        self.assertEqual(
            pick_schema["required"],
            ["item_id", "comment", "reason_tags"],
        )

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
        self.assertIn("職業", prompt)
        self.assertIn("経験", prompt)
        self.assertIn("性別", prompt)
        self.assertIn("年代感", prompt)
        self.assertIn("一人称", prompt)
        self.assertIn("話し方", prompt)
        self.assertIn("価値観", prompt)
        self.assertIn("嫌いなもの", prompt)
        self.assertIn("客観的な無味乾燥レビューではなく", prompt)
        self.assertIn("この人ならこう感じる", prompt)
        self.assertIn("他のキャラクター名を名乗らない", prompt)
        self.assertIn("自分を名乗るなら", prompt)
        self.assertIn("一人称は", prompt)
        self.assertIn("別ペルソナの名前", prompt)

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
        self.assertIn("ツッコミ", prompt)
        self.assertIn("呆れ気味", prompt)
        self.assertIn("不快・攻撃的・見下し表現は禁止", prompt)
        self.assertIn("人ではなく話題や状況に対して毒づく", prompt)

    def test_build_briefing_navigator_task_makes_persona_values_explicit(self):
        task = build_briefing_navigator_task(
            persona="analyst",
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
        self.assertIn("良いと感じる点", prompt)
        self.assertIn("引っかかる点", prompt)
        self.assertIn("今読む理由", prompt)
        self.assertIn("ペルソナの価値観に基づいて選ぶ", prompt)

    def test_build_briefing_navigator_task_allows_intro_only_when_no_candidates(self):
        task = build_briefing_navigator_task(
            persona="native",
            candidates=[],
            intro_context={
                "now_jst": "2026-03-23T19:30:00+09:00",
                "date_jst": "2026-03-23",
                "weekday_jst": "Monday",
                "time_of_day": "evening",
                "season_hint": "early_spring",
            },
        )

        prompt = task["prompt"]
        self.assertIn("picks は空配列 [] を返す", prompt)
        self.assertIn("記事推薦は捏造しない", prompt)
        self.assertIn("candidates が空のときは", prompt)

    def test_ask_navigator_schema_requires_all_fields(self):
        self.assertEqual(
            ASK_NAVIGATOR_SCHEMA["required"],
            ["headline", "commentary", "next_angles"],
        )

    def test_build_ask_navigator_task_forces_premise_and_next_angles(self):
        task = build_ask_navigator_task(
            persona="native",
            ask_input={
                "query": "今週のAI業界で本当に見るべき論点は？",
                "answer": "回答本文",
                "bullets": ["論点1", "論点2"],
                "citations": [{"item_id": "item-1", "title": "記事1", "url": "https://example.com/1"}],
                "related_items": [{"item_id": "item-1", "title": "記事1", "url": "https://example.com/1", "summary": "summary"}],
            },
        )

        prompt = task["prompt"]
        self.assertIn("前提・留保・次に掘る論点", prompt)
        self.assertIn("5〜8文", prompt)
        self.assertIn("回答の要約や言い換えを主目的にしない", prompt)
        self.assertIn("next_angles", prompt)
        self.assertIn("このペルソナの主観", prompt)
        self.assertIn("他のキャラクター名を名乗らない", prompt)
        self.assertIn("一人称は", prompt)
        self.assertIn("次にどこを見るべきか", prompt)

    def test_source_navigator_schema_requires_all_sections(self):
        self.assertEqual(
            SOURCE_NAVIGATOR_SCHEMA["required"],
            ["overview", "keep", "watch", "standout"],
        )

    def test_build_source_navigator_task_requires_long_overview_and_structured_lists(self):
        task = build_source_navigator_task(
            persona="editor",
            candidates=[
                {
                    "source_id": "source-1",
                    "title": "Example Source",
                    "url": "https://example.com/feed",
                    "enabled": True,
                    "status": "ok",
                    "total_items_30d": 14,
                    "unread_items_30d": 4,
                    "read_items_30d": 10,
                    "favorite_count_30d": 3,
                    "avg_items_per_day_30d": 0.5,
                    "active_days_30d": 12,
                    "avg_items_per_active_day_30d": 1.2,
                    "failure_rate": 0.0,
                }
            ],
        )

        prompt = task["prompt"]
        self.assertIn("6〜10文", prompt)
        self.assertIn("総評", prompt)
        self.assertIn("keep", prompt)
        self.assertIn("watch", prompt)
        self.assertIn("standout", prompt)
        self.assertIn("客観的レポートではなく", prompt)
        self.assertIn("数字をそのまま列挙するだけで終わらせず", prompt)
        self.assertIn("同じ source_id を複数カテゴリに重複させない", prompt)


if __name__ == "__main__":
    unittest.main()
