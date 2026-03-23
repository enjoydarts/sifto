import unittest

from app.services.feed_task_common import ITEM_NAVIGATOR_SCHEMA, build_item_navigator_task


class ItemNavigatorTaskCommonTests(unittest.TestCase):
    def test_item_navigator_schema_requires_all_fields_for_strict_json_schema(self):
        self.assertEqual(
            ITEM_NAVIGATOR_SCHEMA["required"],
            ["headline", "commentary", "stance_tags"],
        )

    def test_build_item_navigator_task_includes_commentary_structure_rules(self):
        task = build_item_navigator_task(
            persona="editor",
            article={
                "item_id": "item-1",
                "title": "Example title",
                "translated_title": "翻訳タイトル",
                "source_title": "Example Source",
                "summary": "Summary text",
                "facts": ["Fact one", "Fact two"],
                "published_at": "2026-03-23T19:30:00+09:00",
            },
        )

        prompt = task["prompt"]
        self.assertIn("4〜7文", prompt)
        self.assertIn("summary と facts を土台", prompt)
        self.assertIn("なぜ気にする価値があるか", prompt)
        self.assertIn("どこを少し警戒するか", prompt)
        self.assertIn("headline", prompt)
        self.assertIn("commentary", prompt)
        self.assertIn("記事の内容説明ではなく", prompt)
        self.assertIn("要約の要約", prompt)
        self.assertIn("読みどころ", prompt)
        self.assertIn("違和感", prompt)
        self.assertIn("職業", prompt)
        self.assertIn("経験", prompt)
        self.assertIn("性別", prompt)
        self.assertIn("年代感", prompt)
        self.assertIn("一人称", prompt)
        self.assertIn("話し方", prompt)
        self.assertIn("価値観", prompt)
        self.assertIn("嫌いなもの", prompt)
        self.assertIn("客観的レビューではなく", prompt)
        self.assertIn("この人ならこう感じる", prompt)
        self.assertIn("どう行動するか", prompt)
        self.assertIn("他のキャラクター名を名乗らない", prompt)
        self.assertIn("自分を名乗るなら", prompt)
        self.assertIn("一人称は", prompt)
        self.assertIn("別ペルソナの名前", prompt)

    def test_build_item_navigator_task_keeps_snark_guardrails(self):
        task = build_item_navigator_task(
            persona="snark",
            article={
                "item_id": "item-1",
                "title": "Example title",
                "translated_title": "翻訳タイトル",
                "source_title": "Example Source",
                "summary": "Summary text",
                "facts": ["Fact one", "Fact two"],
                "published_at": "2026-03-23T19:30:00+09:00",
            },
        )

        prompt = task["prompt"]
        self.assertIn("軽口", prompt)
        self.assertIn("ツッコミ", prompt)
        self.assertIn("不快・攻撃的・見下し表現は禁止", prompt)
        self.assertIn("人ではなく話題や状況に対して毒づく", prompt)

    def test_build_item_navigator_task_forces_subjective_axes(self):
        task = build_item_navigator_task(
            persona="concierge",
            article={
                "item_id": "item-1",
                "title": "Example title",
                "translated_title": "翻訳タイトル",
                "source_title": "Example Source",
                "summary": "Summary text",
                "facts": ["Fact one", "Fact two"],
                "published_at": "2026-03-23T19:30:00+09:00",
            },
        )

        prompt = task["prompt"]
        self.assertIn("第一印象", prompt)
        self.assertIn("良いと感じた点", prompt)
        self.assertIn("微妙だと感じた点", prompt)
        self.assertIn("気になったポイント", prompt)


if __name__ == "__main__":
    unittest.main()
