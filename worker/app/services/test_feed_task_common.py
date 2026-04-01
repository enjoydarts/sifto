import unittest
from unittest.mock import patch

from app.services.feed_task_common import (
    AUDIO_BRIEFING_SCRIPT_SCHEMA,
    AUDIO_BRIEFING_CHARS_PER_MINUTE,
    ASK_NAVIGATOR_SCHEMA,
    BRIEFING_NAVIGATOR_SCHEMA,
    SOURCE_NAVIGATOR_SCHEMA,
    is_audio_briefing_script_retryable_validation_error,
    resolve_navigator_sampling_profile,
    _resolve_persona_file,
    _audio_briefing_article_section_budgets,
    _audio_briefing_duo_article_turn_count,
    _audio_briefing_script_budgets,
    build_audio_briefing_script_schema,
    build_audio_briefing_script_task,
    build_ask_navigator_task,
    build_briefing_navigator_task,
    build_item_navigator_task,
    build_source_navigator_task,
    parse_audio_briefing_script_result,
)
from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import bind_prompt_override


class FeedTaskCommonTests(unittest.TestCase):
    def test_build_audio_briefing_script_schema_for_duo_opening_requires_turns_shape(self):
        schema = build_audio_briefing_script_schema(
            conversation_mode="duo",
            include_opening=True,
            include_overall_summary=False,
            include_article_segments=False,
            include_ending=False,
            article_count=0,
        )

        self.assertEqual(schema["required"], ["turns"])
        turns = schema["properties"]["turns"]
        self.assertEqual(turns["minItems"], 5)
        items = turns["items"]
        self.assertEqual(items["properties"]["speaker"]["enum"], ["host", "partner"])
        self.assertEqual(items["properties"]["section"]["enum"], ["opening"])
        self.assertNotIn("item_id", items["properties"])

    def test_build_audio_briefing_script_schema_for_duo_article_requires_turns_shape(self):
        schema = build_audio_briefing_script_schema(
            conversation_mode="duo",
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
            article_count=5,
            article_turn_count=5,
        )

        self.assertEqual(schema["required"], ["turns"])
        turns = schema["properties"]["turns"]
        self.assertEqual(turns["minItems"], 25)
        items = turns["items"]
        self.assertEqual(items["properties"]["section"]["enum"], ["article"])
        self.assertIn("item_id", items["required"])

    def test_build_audio_briefing_script_schema_for_duo_article_uses_three_turn_mode_when_requested(self):
        schema = build_audio_briefing_script_schema(
            conversation_mode="duo",
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
            article_count=5,
            article_turn_count=3,
        )

        turns = schema["properties"]["turns"]
        self.assertEqual(turns["minItems"], 15)

    def test_build_audio_briefing_script_schema_for_duo_mixed_sections_requires_item_id_only_for_article(self):
        schema = build_audio_briefing_script_schema(
            conversation_mode="duo",
            include_opening=True,
            include_overall_summary=True,
            include_article_segments=True,
            include_ending=True,
            article_count=2,
            article_turn_count=3,
        )

        turns = schema["properties"]["turns"]
        self.assertEqual(turns["minItems"], 21)
        item_schema = turns["items"]
        self.assertIn("anyOf", item_schema)
        self.assertEqual(len(item_schema["anyOf"]), 2)
        frame_schema = item_schema["anyOf"][0]
        article_schema = item_schema["anyOf"][1]
        self.assertNotIn("item_id", frame_schema["required"])
        self.assertIn("item_id", article_schema["required"])

    def test_audio_briefing_duo_article_turn_count_switches_by_budget(self):
        self.assertEqual(_audio_briefing_duo_article_turn_count(419), 3)
        self.assertEqual(_audio_briefing_duo_article_turn_count(420), 5)

    def test_audio_briefing_article_section_budgets_fit_within_article_budget(self):
        _opening_budget, _summary_budget, _ending_budget, article_budget = _audio_briefing_script_budgets(12000, 30)
        headline_budget, summary_intro_budget, commentary_budget = _audio_briefing_article_section_budgets(article_budget)

        self.assertEqual(article_budget, 328)
        self.assertEqual(headline_budget + summary_intro_budget + commentary_budget, article_budget)
        self.assertLessEqual(headline_budget, article_budget)
        self.assertLessEqual(summary_intro_budget, article_budget)
        self.assertLessEqual(commentary_budget, article_budget)

    def test_audio_briefing_script_budgets_do_not_reserve_frame_budget_for_article_only_batches(self):
        opening_budget, summary_budget, ending_budget, article_budget = _audio_briefing_script_budgets(
            1104,
            3,
            include_opening=False,
            include_overall_summary=False,
            include_ending=False,
        )

        self.assertEqual(opening_budget, 0)
        self.assertEqual(summary_budget, 0)
        self.assertEqual(ending_budget, 0)
        self.assertGreater(article_budget, 300)

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

    def test_resolve_navigator_sampling_profile_uses_persona_defaults(self):
        profile = resolve_navigator_sampling_profile("hype")

        self.assertEqual(profile["temperature_hint"], "medium_high")
        self.assertEqual(profile["top_p_hint"], "wide")
        self.assertEqual(profile["verbosity_hint"], "expansive")
        self.assertEqual(profile["temperature"], 0.7)
        self.assertEqual(profile["top_p"], 0.98)

    def test_build_item_navigator_task_exposes_sampling_and_verbosity(self):
        task = build_item_navigator_task(
            persona="analyst",
            article={
                "item_id": "item-1",
                "title": "Example title",
                "translated_title": "翻訳タイトル",
                "summary": "Summary text",
                "facts": ["Fact 1", "Fact 2"],
            },
        )

        self.assertEqual(task["sampling_profile"]["temperature_hint"], "low")
        self.assertEqual(task["sampling_profile"]["top_p_hint"], "narrow")
        self.assertEqual(task["sampling_profile"]["verbosity_hint"], "tight")
        self.assertEqual(task["sampling_profile"]["temperature"], 0.2)
        self.assertEqual(task["sampling_profile"]["top_p"], 0.75)
        self.assertIn("簡潔寄り", task["prompt"])

    def test_audio_briefing_script_schema_requires_all_fields(self):
        self.assertEqual(
            AUDIO_BRIEFING_SCRIPT_SCHEMA["required"],
            ["opening", "overall_summary", "article_segments", "ending"],
        )

    def test_build_audio_briefing_script_task_uses_full_persona_and_fixed_article_order(self):
        task = build_audio_briefing_script_task(
            persona="analyst",
            articles=[
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "source_title": "Example Source",
                    "summary": "Summary text",
                    "published_at": "2026-03-23T19:30:00+09:00",
                }
            ],
            intro_context={
                "now_jst": "2026-03-23T19:30:00+09:00",
                "date_jst": "2026-03-23",
                "weekday_jst": "Monday",
                "time_of_day": "evening",
                "season_hint": "early_spring",
            },
            target_duration_minutes=20,
            target_chars=14000,
            chars_per_minute=AUDIO_BRIEFING_CHARS_PER_MINUTE,
        )

        prompt = task["prompt"]
        self.assertIn("単独話者のAIナビゲーター", prompt)
        self.assertIn("音声ブリーフィング台本", prompt)
        self.assertIn("article_segments 有効時は入力 articles と同じ順番・同じ件数で返してください", prompt)
        self.assertIn("職業", prompt)
        self.assertIn("経験", prompt)
        self.assertIn("性別", prompt)
        self.assertIn("年代感", prompt)
        self.assertIn("一人称", prompt)
        self.assertIn("話し方", prompt)
        self.assertIn("価値観", prompt)
        self.assertIn("嫌いなもの", prompt)
        self.assertIn("intro_style", prompt)
        self.assertIn("item_style_hint", prompt)
        self.assertIn("客観的な無味乾燥レビューではなく", prompt)
        self.assertIn("別ペルソナの名前・肩書き・口調", prompt)
        self.assertIn("目標尺: 約 20 分", prompt)
        self.assertIn("今回返すセクション全体の目標文字数: 約 14000 文字", prompt)
        self.assertIn(f"1分あたり {AUDIO_BRIEFING_CHARS_PER_MINUTE} 文字", prompt)
        self.assertIn("全体の本文合計は必ず 12600文字以上 15400文字以下にしてください", prompt)
        self.assertIn("opening 有効時は 10〜12文 / 630〜805文字", prompt)
        self.assertIn("overall_summary 有効時は 13〜14文 / 882〜1127文字", prompt)
        self.assertIn("ending 有効時は 10〜12文 / 630〜805文字", prompt)
        self.assertIn("headline は 3〜5文 / 144〜184文字", prompt)
        self.assertIn("summary_intro は 9〜10文 / 414〜529文字", prompt)
        self.assertIn("commentary は 13〜14文 / 9810〜12535文字", prompt)
        self.assertIn("1文は 60〜110文字 を目安にしつつ", prompt)
        self.assertIn("commentary では headline や summary_intro の言い換えで終わらず", prompt)
        self.assertIn("headline でこれから扱う記事をリスナーに詳細に紹介し、commentary でそのペルソナの反応を書く", prompt)
        self.assertIn("全セクションで1文ごとに改行", prompt)
        self.assertNotIn("{{task_block}}", prompt)

    def test_build_audio_briefing_script_task_omits_unrequested_sections(self):
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=[
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "source_title": "Example Source",
                    "summary": "Summary text",
                }
            ],
            intro_context={},
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
        )

        prompt = task["prompt"]
        self.assertIn("article_segments 有効時は入力 articles と同じ順番・同じ件数で返してください", prompt)
        self.assertNotIn('"opening": "導入"', prompt)
        self.assertNotIn('"overall_summary": "全体サマリー"', prompt)
        self.assertNotIn('"ending": "締め"', prompt)

    def test_build_audio_briefing_script_task_uses_tighter_ranges_for_dense_episode(self):
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=[
                {"item_id": f"item-{i}", "title": f"Title {i}", "translated_title": f"翻訳{i}", "summary": "Summary text"}
                for i in range(1, 21)
            ],
            intro_context={},
            target_duration_minutes=30,
            target_chars=12000,
            chars_per_minute=AUDIO_BRIEFING_CHARS_PER_MINUTE,
        )

        prompt = task["prompt"]
        self.assertIn("headline は 2〜3文 / 53〜73文字", prompt)
        self.assertIn("summary_intro は 4〜6文 / 191〜244文字", prompt)
        self.assertIn("commentary は 5〜7文 / 200〜255文字", prompt)
        self.assertIn("1文は 50〜95文字 を目安", prompt)

    def test_build_audio_briefing_script_task_adds_supplement_instructions_for_existing_section(self):
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=[
                {"item_id": "item-1", "title": "Title 1", "translated_title": "翻訳1", "summary": "Summary text"}
            ],
            intro_context={
                "audio_briefing_generation_mode": "supplement",
                "audio_briefing_generation_section": "opening",
                "audio_briefing_existing_section_text": "おはようございます。\n今日は静かな朝ですね。",
            },
            include_opening=True,
            include_overall_summary=False,
            include_article_segments=False,
            include_ending=False,
        )

        prompt = task["prompt"]
        self.assertIn("opening の不足分を補う追記モード", prompt)
        self.assertIn("差分だけを書く", prompt)
        self.assertIn("既存の opening:", prompt)
        self.assertIn("今日は静かな朝ですね。", prompt)

    def test_build_audio_briefing_script_task_adds_article_supplement_instructions(self):
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=[
                {"item_id": "item-1", "title": "Title 1", "translated_title": "翻訳1", "summary": "Summary text"}
            ],
            intro_context={
                "audio_briefing_generation_mode": "supplement",
                "audio_briefing_generation_section": "article_segments",
                "audio_briefing_existing_article_segments": [
                    {
                        "item_id": "item-1",
                        "headline": "見出しです。",
                        "summary_intro": "要点です。",
                        "commentary": "短い感想です。",
                    }
                ],
            },
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
        )

        prompt = task["prompt"]
        self.assertIn("article_segments の不足分を補う追記モード", prompt)
        self.assertIn("commentary を最優先で厚くし", prompt)
        self.assertIn("既存の article_segments:", prompt)
        self.assertIn('"item_id": "item-1"', prompt)

    def test_build_audio_briefing_script_task_duo_uses_turns_and_two_personas(self):
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=[
                {"item_id": "item-1", "title": "Title 1", "translated_title": "翻訳1", "summary": "Summary text"}
            ],
            intro_context={
                "audio_briefing_conversation_mode": "duo",
                "audio_briefing_host_persona": "editor",
                "audio_briefing_partner_persona": "analyst",
                "now_jst": "2026-03-31T18:30:00+09:00",
                "date_jst": "2026-03-31",
                "weekday_jst": "Tuesday",
                "time_of_day": "evening",
                "season_hint": "spring",
            },
            target_duration_minutes=20,
            target_chars=12000,
            chars_per_minute=AUDIO_BRIEFING_CHARS_PER_MINUTE,
        )

        prompt = task["prompt"]
        self.assertIn("二人組のAIナビゲーター", prompt)
        self.assertIn("host は", prompt)
        self.assertIn("partner は", prompt)
        self.assertIn("turns", prompt)
        self.assertIn("speaker", prompt)
        self.assertIn("section", prompt)
        self.assertIn("host と partner が会話しながら番組を進めます", prompt)
        self.assertIn("opening / overall_summary / ending はそれぞれ 5手で書いてください", prompt)
        self.assertIn("article は今回 5手、並びは host -> partner -> host -> partner -> host です", prompt)
        self.assertIn("article の役割分担は host(setup) -> partner(reaction) -> host(deepen) -> partner(contrast) -> host(close) です", prompt)
        self.assertIn("partner は毎回、直前の host を受けて会話を横に広げてください", prompt)
        self.assertIn("partner の各 turn は、直前の host turn を受けて始めてください", prompt)
        self.assertIn("host の各 turn は、直前の partner turn を軽く受けてから話を前へ進めてください", prompt)
        self.assertIn("今回の対象 section は article", prompt)
        self.assertIn("now_jst: 2026-03-31T18:30:00+09:00", prompt)
        self.assertIn("time_of_day: evening", prompt)
        self.assertNotIn("単独話者のAIナビゲーター", prompt)
        self.assertNotIn("article_segments の各 headline", prompt)
        self.assertNotIn('"article_segments": [', prompt)

    def test_build_audio_briefing_script_task_duo_uses_three_turn_article_mode_when_budget_is_tight(self):
        articles = [
            {"item_id": f"item-{idx}", "title": f"Title {idx}", "translated_title": f"翻訳{idx}", "summary": "Summary text"}
            for idx in range(1, 21)
        ]
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=articles,
            intro_context={
                "audio_briefing_conversation_mode": "duo",
                "audio_briefing_host_persona": "editor",
                "audio_briefing_partner_persona": "analyst",
                "audio_briefing_generation_section": "article_segments",
                "audio_briefing_program_position": "article_midstream",
                "audio_briefing_article_batch_start_index": 1,
                "audio_briefing_article_batch_end_index": 3,
                "audio_briefing_total_articles": 20,
            },
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
            target_duration_minutes=20,
            target_chars=8000,
            chars_per_minute=AUDIO_BRIEFING_CHARS_PER_MINUTE,
        )

        prompt = task["prompt"]
        self.assertIn("article は今回 3手、並びは host -> partner -> host です", prompt)
        self.assertIn("article の役割分担は host(setup) -> partner(reaction/contrast) -> host(close) です", prompt)
        self.assertIn("article は各記事を 3手 の中で完結させ", prompt)
        self.assertEqual(task["schema"]["properties"]["turns"]["minItems"], 60)
        self.assertNotIn("article_segments の各 headline", prompt)

    def test_build_audio_briefing_script_task_duo_article_batch_includes_program_position(self):
        task = build_audio_briefing_script_task(
            persona="editor",
            articles=[
                {"item_id": "item-1", "title": "Title 1", "translated_title": "翻訳1", "summary": "Summary text"},
                {"item_id": "item-2", "title": "Title 2", "translated_title": "翻訳2", "summary": "Summary text"},
                {"item_id": "item-3", "title": "Title 3", "translated_title": "翻訳3", "summary": "Summary text"},
            ],
            intro_context={
                "audio_briefing_conversation_mode": "duo",
                "audio_briefing_host_persona": "editor",
                "audio_briefing_partner_persona": "analyst",
                "audio_briefing_generation_section": "article_segments",
                "audio_briefing_program_position": "article_midstream",
                "audio_briefing_article_batch_start_index": 4,
                "audio_briefing_article_batch_end_index": 6,
                "audio_briefing_total_articles": 20,
            },
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
            target_duration_minutes=20,
            target_chars=12000,
            chars_per_minute=AUDIO_BRIEFING_CHARS_PER_MINUTE,
        )

        prompt = task["prompt"]
        self.assertIn("今回は番組の article パートだけを書く", prompt)
        self.assertIn("今回の対象 section は article", prompt)
        self.assertIn("program_position: article_midstream", prompt)
        self.assertIn("article_batch_range: 4 - 6", prompt)
        self.assertIn("今回の batch は全20本中の 4本目から6本目までを担当する", prompt)
        self.assertIn("opening と overall_summary はすでに終わっている前提で続きから入る", prompt)
        self.assertIn("各記事の最後は完全に閉じすぎず、次の話題へ滑らかにつながる余地を残す", prompt)
        self.assertIn("batch の最後のやり取りは、ここで番組全体を締めず", prompt)

    def test_parse_audio_briefing_script_result_accepts_duo_turns_only(self):
        result = parse_audio_briefing_script_result(
            """
            {
              "turns": [
                {"speaker": "host", "section": "opening", "text": "おはようございます。今日は二人で見ていきます。"},
                {"speaker": "partner", "section": "opening", "text": "朝の一本目としては、ちょっと面白い並びですね。"},
                {"speaker": "host", "section": "article", "item_id": "item-1", "text": "まずはこの記事です。"},
                {"speaker": "partner", "section": "article", "item_id": "item-1", "text": "ここは見方の違いが出そうです。"},
                {"speaker": "host", "section": "ending", "text": "では今日はこのへんで。"}
              ]
            }
            """,
            [
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "summary": "Summary text",
                }
            ],
            "editor",
            conversation_mode="duo",
        )
        self.assertEqual(result["opening"], "")
        self.assertEqual(result["overall_summary"], "")
        self.assertEqual(result["ending"], "")
        self.assertEqual(result["article_segments"], [])
        self.assertEqual(len(result["turns"]), 5)
        self.assertEqual(result["turns"][0]["speaker"], "host")
        self.assertEqual(result["turns"][2]["item_id"], "item-1")

    def test_parse_audio_briefing_script_result_rejects_empty_payload(self):
        with self.assertRaises(ValueError):
            parse_audio_briefing_script_result(
                "{}",
                [
                    {
                        "item_id": "item-1",
                        "title": "Example title",
                        "translated_title": "翻訳タイトル",
                        "summary": "Summary text",
                    }
                ],
                "editor",
            )

    def test_parse_audio_briefing_script_result_rejects_missing_turns_for_duo(self):
        with self.assertRaisesRegex(ValueError, "audio briefing script missing turns"):
            parse_audio_briefing_script_result(
                "{}",
                [
                    {
                        "item_id": "item-1",
                        "title": "Example title",
                        "translated_title": "翻訳タイトル",
                        "summary": "Summary text",
                    }
                ],
                "editor",
                conversation_mode="duo",
            )

    def test_parse_audio_briefing_script_result_reports_empty_turns_state_for_duo(self):
        with self.assertRaisesRegex(ValueError, r"state=list\(len=0\)"):
            parse_audio_briefing_script_result(
                '{"turns":[]}',
                [
                    {
                        "item_id": "item-1",
                        "title": "Example title",
                        "translated_title": "翻訳タイトル",
                        "summary": "Summary text",
                    }
                ],
                "editor",
                conversation_mode="duo",
            )

    def test_parse_audio_briefing_script_result_single_rejects_turns_only_payload(self):
        with self.assertRaisesRegex(ValueError, "audio briefing script missing opening"):
            parse_audio_briefing_script_result(
                """
                {
                  "turns": [
                    {"speaker": "host", "section": "opening", "text": "おはようございます。"}
                  ]
                }
                """,
                [
                    {
                        "item_id": "item-1",
                        "title": "Example title",
                        "translated_title": "翻訳タイトル",
                        "summary": "Summary text",
                    }
                ],
                "editor",
                conversation_mode="single",
            )

    def test_parse_audio_briefing_script_result_rejects_missing_commentary(self):
        with self.assertRaises(ValueError):
            parse_audio_briefing_script_result(
                """
                {
                  "opening": "導入です。",
                  "overall_summary": "全体まとめです。",
                  "article_segments": [
                    {
                      "item_id": "item-1",
                      "headline": "見出し",
                      "summary_intro": "要約です。"
                    }
                  ],
                  "ending": "締めです。"
                }
                """,
                [
                    {
                        "item_id": "item-1",
                        "title": "Example title",
                        "translated_title": "翻訳タイトル",
                        "summary": "Summary text",
                    }
                ],
                "editor",
            )

    def test_audio_briefing_script_retryable_validation_error_detects_count_mismatch(self):
        self.assertTrue(
            is_audio_briefing_script_retryable_validation_error(
                ValueError("audio briefing script article_segments count mismatch")
            )
        )

    def test_audio_briefing_script_retryable_validation_error_excludes_input_errors(self):
        self.assertFalse(
            is_audio_briefing_script_retryable_validation_error(
                ValueError("audio briefing input article missing item_id at index 1")
            )
        )

    def test_parse_audio_briefing_script_result_allows_article_only_sections(self):
        result = parse_audio_briefing_script_result(
            """
            {
              "article_segments": [
                {
                  "item_id": "item-1",
                  "headline": "見出し",
                  "summary_intro": "要約です。",
                  "commentary": "コメントです。"
                }
              ]
            }
            """,
            [
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "summary": "Summary text",
                }
            ],
            "editor",
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
        )
        self.assertEqual(result["opening"], "")
        self.assertEqual(result["overall_summary"], "")
        self.assertEqual(result["ending"], "")
        self.assertEqual(len(result["article_segments"]), 1)

    def test_parse_audio_briefing_script_result_uses_article_order_when_item_ids_mismatch(self):
        result = parse_audio_briefing_script_result(
            """
            {
              "article_segments": [
                {
                  "item_id": "wrong-item-id",
                  "headline": "見出しA",
                  "summary_intro": "要約Aです。",
                  "commentary": "コメントAです。"
                },
                {
                  "item_id": "another-wrong-id",
                  "headline": "見出しB",
                  "summary_intro": "要約Bです。",
                  "commentary": "コメントBです。"
                }
              ]
            }
            """,
            [
                {
                    "item_id": "item-1",
                    "title": "Example title 1",
                    "translated_title": "翻訳タイトル1",
                    "summary": "Summary text 1",
                },
                {
                    "item_id": "item-2",
                    "title": "Example title 2",
                    "translated_title": "翻訳タイトル2",
                    "summary": "Summary text 2",
                },
            ],
            "editor",
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
        )
        self.assertEqual(result["article_segments"][0]["item_id"], "item-1")
        self.assertEqual(result["article_segments"][0]["headline"], "見出しA")
        self.assertEqual(result["article_segments"][1]["item_id"], "item-2")
        self.assertEqual(result["article_segments"][1]["headline"], "見出しB")

    def test_parse_audio_briefing_script_result_scales_caps_for_long_targets(self):
        long_summary = "総括です。" * 800
        long_commentary = "コメントです。" * 400
        result = parse_audio_briefing_script_result(
            f"""
            {{
              "opening": "導入です。",
              "overall_summary": "{long_summary}",
              "article_segments": [
                {{
                  "item_id": "item-1",
                  "headline": "見出しA",
                  "summary_intro": "要約Aです。",
                  "commentary": "{long_commentary}"
                }}
              ],
              "ending": "締めです。"
            }}
            """,
            [
                {
                    "item_id": "item-1",
                    "title": "Example title 1",
                    "translated_title": "翻訳タイトル1",
                    "summary": "Summary text 1",
                }
            ],
            "editor",
            target_chars=14000,
        )
        self.assertGreaterEqual(len(result["overall_summary"]), 980)
        self.assertGreater(len(result["article_segments"][0]["commentary"]), 1200)

    def test_parse_audio_briefing_script_result_preserves_full_frame_sections(self):
        long_opening = "導入です。" * 300
        long_summary = "総括です。" * 500
        long_ending = "締めです。" * 300
        result = parse_audio_briefing_script_result(
            f"""
            {{
              "opening": "{long_opening}",
              "overall_summary": "{long_summary}",
              "article_segments": [
                {{
                  "item_id": "item-1",
                  "headline": "見出しA",
                  "summary_intro": "要約Aです。",
                  "commentary": "コメントAです。"
                }}
              ],
              "ending": "{long_ending}"
            }}
            """,
            [
                {
                    "item_id": "item-1",
                    "title": "Example title 1",
                    "translated_title": "翻訳タイトル1",
                    "summary": "Summary text 1",
                }
            ],
            "editor",
            target_chars=14000,
        )
        self.assertEqual(result["opening"], long_opening)
        self.assertEqual(result["overall_summary"], long_summary)
        self.assertEqual(result["ending"], long_ending)

    def test_parse_audio_briefing_script_result_preserves_commentary_line_breaks(self):
        result = parse_audio_briefing_script_result(
            """
            {
              "article_segments": [
                {
                  "item_id": "item-1",
                  "headline": "見出し",
                  "summary_intro": "要約です。",
                  "commentary": "一文目です。\\n二文目です。\\n三文目です。"
                }
              ]
            }
            """,
            [
                {
                    "item_id": "item-1",
                    "title": "Example title",
                    "translated_title": "翻訳タイトル",
                    "summary": "Summary text",
                }
            ],
            "editor",
            include_opening=False,
            include_overall_summary=False,
            include_article_segments=True,
            include_ending=False,
        )
        self.assertEqual(result["article_segments"][0]["commentary"], "一文目です。\n二文目です。\n三文目です。")

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

    def test_default_template_override_matches_code_default_rendering_for_audio_briefing(self):
        articles = [
            {
                "item_id": "item-1",
                "title": "Example title",
                "translated_title": "翻訳タイトル",
                "source_title": "Source",
                "summary": "Summary text",
                "published_at": "2026-04-01T08:00:00Z",
            }
        ]
        intro_context = {
            "now_jst": "2026-04-01T19:30:00+09:00",
            "date_jst": "2026-04-01",
            "weekday_jst": "Wednesday",
            "time_of_day": "evening",
            "season_hint": "spring",
        }
        expected = build_audio_briefing_script_task(
            persona="editor",
            articles=articles,
            intro_context=intro_context,
            target_duration_minutes=10,
            target_chars=4000,
        )
        default_template = get_default_prompt_template("audio_briefing_script.single")

        with bind_prompt_override(
            "audio_briefing_script.single",
            default_template["prompt_text"],
            default_template["system_instruction"],
        ):
            actual = build_audio_briefing_script_task(
                persona="editor",
                articles=articles,
                intro_context=intro_context,
                target_duration_minutes=10,
                target_chars=4000,
            )

        self.assertEqual(actual["system_instruction"], expected["system_instruction"])
        self.assertEqual(actual["prompt"], expected["prompt"])


if __name__ == "__main__":
    unittest.main()
