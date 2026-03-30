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
    _audio_briefing_script_budgets,
    build_audio_briefing_script_task,
    build_ask_navigator_task,
    build_briefing_navigator_task,
    build_item_navigator_task,
    build_source_navigator_task,
    parse_audio_briefing_script_result,
)


class FeedTaskCommonTests(unittest.TestCase):
    def test_audio_briefing_article_section_budgets_fit_within_article_budget(self):
        _opening_budget, _summary_budget, _ending_budget, article_budget = _audio_briefing_script_budgets(12000, 30)
        headline_budget, summary_intro_budget, commentary_budget = _audio_briefing_article_section_budgets(article_budget)

        self.assertEqual(article_budget, 328)
        self.assertEqual(headline_budget + summary_intro_budget + commentary_budget, article_budget)
        self.assertLessEqual(headline_budget, article_budget)
        self.assertLessEqual(summary_intro_budget, article_budget)
        self.assertLessEqual(commentary_budget, article_budget)

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
        self.assertIn("article_segments は入力 articles と同じ順番・同じ件数", prompt)
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
        self.assertIn("今回返すセクションの目標文字数: 約 14000 文字", prompt)
        self.assertIn(f"1分あたり {AUDIO_BRIEFING_CHARS_PER_MINUTE} 文字", prompt)
        self.assertIn("opening は 10〜12文", prompt)
        self.assertIn("少なくとも 10文は使い", prompt)
        self.assertIn("opening は番組のオープニングトークとして書く。リスナーに向かって語りかけ", prompt)
        self.assertIn("挨拶、時間帯や季節感、軽い日常雑談", prompt)
        self.assertIn("opening では個別記事の紹介を始めない", prompt)
        self.assertIn("企業名、製品名、出来事、具体的ニュース内容に触れない", prompt)
        self.assertIn("overall_summary は総括であり、13〜14文", prompt)
        self.assertIn("少なくとも 13文は使い", prompt)
        self.assertIn("ending は番組を終わらせる締めの言葉として 10〜12文", prompt)
        self.assertIn("少なくとも 10文は使い", prompt)
        self.assertIn("overall_summary は総括", prompt)
        self.assertIn("記事の順番紹介", prompt)
        self.assertIn("記事の1行要約を機械的に並べない", prompt)
        self.assertIn("共通テーマ、対立軸、温度感、いま追う意味、記事間のつながりを広く語ってよい", prompt)
        self.assertIn("共通点、流れ、温度差、見方の違い、別の話に見える記事どうしのつながりまで具体的にしてよい", prompt)
        self.assertIn("headline は 3〜4文", prompt)
        self.assertIn("summary_intro は 4〜6文", prompt)
        self.assertIn("commentary は 9〜10文", prompt)
        self.assertIn("少なくとも 3文は使い", prompt)
        self.assertIn("少なくとも 4文は使い", prompt)
        self.assertIn("少なくとも 9文は使い", prompt)
        self.assertIn("長さは約", prompt)
        self.assertIn("1記事あたりの headline と summary_intro と commentary の合計は約", prompt)
        self.assertIn("headline の個別目安: 約", prompt)
        self.assertIn("summary_intro の個別目安: 約", prompt)
        self.assertIn("commentary の個別目安: 約", prompt)
        self.assertIn("必要なら少し超えてよい", prompt)
        self.assertIn("各記事にほぼ均等に尺を配る", prompt)
        self.assertIn("target_chars と記事数から逆算した尺配分を守り", prompt)
        self.assertIn("1文は 60〜110文字 を目安", prompt)
        self.assertIn("無個性な書き方をしない", prompt)
        self.assertIn("このペルソナならどう受け取るか", prompt)
        self.assertIn("headline では、その記事をリスナーに詳細に紹介するつもりで話す。何の記事で、何が起きていて、どこが気になるのかが自然に伝わるようにする", prompt)
        self.assertIn("headline は短い導入見出しとして使い", prompt)
        self.assertIn("summary_intro では、記事の要点、何が起きたか、どこを見る記事かをやや詳しく要約してよい。", prompt)
        self.assertIn("反応の理由、軽い背景説明、比較、自分ならどう見るかまで話してよい", prompt)
        self.assertIn("要点を置いたら、反応、理由、比較に進む", prompt)
        self.assertIn("解説調に見えやすい運びを避ける", prompt)
        self.assertIn("headline でこれから扱う記事をリスナーに詳細に紹介し、summary_intro で記事の中身を置いたうえで、このペルソナなら何に反応するかを話す", prompt)
        self.assertIn("ending は番組を終わらせる締めの言葉", prompt)
        self.assertIn("今日の回で残った感触や温度感を1〜2点だけ軽く振り返る", prompt)
        self.assertIn("最後に残った印象や引っかかりを必ず言葉にする", prompt)
        self.assertIn("聞いてくれたことへの一言と、次の時間へ戻っていく感じを必ず入れる", prompt)
        self.assertIn("85% 未満まで縮めない", prompt)
        self.assertIn("article_segments は各記事の持ち分を使い切る意識", prompt)
        self.assertIn("全セクションで1文ごとに改行", prompt)
        self.assertIn("article commentary でも1文ごとに改行", prompt)

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
        self.assertIn("article_segments は入力 articles と同じ順番・同じ件数", prompt)
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
        self.assertIn("headline は 1〜3文", prompt)
        self.assertIn("summary_intro は 2〜4文", prompt)
        self.assertIn("commentary は 3〜5文", prompt)
        self.assertIn("1文は 50〜95文字 を目安", prompt)
        self.assertIn("commentary の個別目安: 約", prompt)

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


if __name__ == "__main__":
    unittest.main()
