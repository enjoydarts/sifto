import unittest
from unittest.mock import patch

from app.services.tts_markup_preprocess import (
    DEFAULT_TTS_MARKUP_PREPROCESS_PROMPT_KEY,
    ELEVENLABS_TTS_PREPROCESS_PURPOSE,
    FISH_PREPROCESS_PURPOSE,
    GEMINI_TTS_PREPROCESS_PURPOSE,
    TTSMarkupPreprocessService,
    XAI_TTS_PREPROCESS_PURPOSE,
)


class TTSMarkupPreprocessServiceTests(unittest.TestCase):
    def test_preprocess_uses_openai_compatible_transport(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.openai_chat_json",
                return_value=("[自然に]前処理済み", {"input_tokens": 11, "output_tokens": 22}),
            ) as chat_json,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="gpt-5.4-mini",
                api_key="openai-key",
            )

        chat_json.assert_called_once_with(
            "元テキスト",
            "gpt-5.4-mini",
            "openai-key",
            system_instruction="SYSTEM",
            max_output_tokens=3200,
        )
        self.assertEqual(result["text"], "[自然に]前処理済み")
        self.assertEqual(result["llm"]["provider"], "openai")
        self.assertEqual(result["llm"]["model"], "gpt-5.4-mini")

    def test_preprocess_appends_text_when_prompt_has_no_placeholder(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "固定プロンプト\n## 【テキスト】",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.openai_chat_json",
                return_value=("整形済み", {"input_tokens": 8, "output_tokens": 12}),
            ) as chat_json,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="gpt-5.4-mini",
                api_key="openai-key",
            )

        chat_json.assert_called_once_with(
            "固定プロンプト\n## 【テキスト】\n\n元テキスト",
            "gpt-5.4-mini",
            "openai-key",
            system_instruction="SYSTEM",
            max_output_tokens=3200,
        )
        self.assertEqual(result["text"], "整形済み")

    def test_preprocess_uses_gemini_plain_text_generation(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.gemini_generate_content",
                return_value=("[落ち着いて]整形済み", {"input_tokens": 10, "output_tokens": 20}),
            ) as generate_content,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="gemini-2.5-flash",
                api_key="google-key",
            )

        generate_content.assert_called_once_with(
            "元テキスト",
            model="gemini-2.5-flash",
            api_key="google-key",
            max_output_tokens=3200,
            system_instruction="SYSTEM",
            response_mime_type="text/plain",
        )
        self.assertEqual(result["text"], "[落ち着いて]整形済み")
        self.assertEqual(result["llm"]["provider"], "google")

    def test_preprocess_uses_elevenlabs_plain_text_generation(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.openrouter_chat_json",
                return_value=("[自然に]整形済み", {"input_tokens": 10, "output_tokens": 20}),
            ) as chat_json,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="openrouter::openai/gpt-5.4-mini",
                api_key="openrouter-key",
                prompt_key="elevenlabs.summary_preprocess",
            )

        chat_json.assert_called_once_with(
            "元テキスト",
            "openrouter::openai/gpt-5.4-mini",
            "openrouter-key",
            system_instruction="SYSTEM",
            max_output_tokens=3200,
        )
        self.assertEqual(result["text"], "[自然に]整形済み")
        self.assertEqual(result["llm"]["provider"], "openrouter")

    def test_preprocess_uses_xai_plain_text_generation(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.xai_chat_json",
                return_value=("[pause]整形済み", {"input_tokens": 9, "output_tokens": 18}),
            ) as chat_json,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="grok-4-fast-non-reasoning",
                api_key="xai-key",
                prompt_key="xai.summary_preprocess",
            )

        chat_json.assert_called_once_with(
            "元テキスト",
            "grok-4-fast-non-reasoning",
            "xai-key",
            system_instruction="SYSTEM",
            max_output_tokens=3200,
        )
        self.assertEqual(result["text"], "[pause]整形済み")
        self.assertEqual(result["llm"]["provider"], "xai")

    def test_preprocess_uses_minimax_anthropic_compatible_transport(self):
        service = TTSMarkupPreprocessService()
        message = type(
            "Message",
            (),
            {
                "content": [type("Content", (), {"text": "[自然に]整形済み"})()],
                "usage": type(
                    "Usage",
                    (),
                    {
                        "input_tokens": 9,
                        "output_tokens": 18,
                        "cache_creation_input_tokens": 0,
                        "cache_read_input_tokens": 0,
                    },
                )(),
            },
        )()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.minimax_call_with_model_fallback",
                return_value=(message, "MiniMax-M2.5", []),
            ) as minimax_call,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="MiniMax-M2.5",
                api_key="minimax-key",
            )

        minimax_call.assert_called_once_with(
            "SYSTEM\n\n元テキスト",
            "MiniMax-M2.5",
            None,
            max_tokens=3200,
            api_key="minimax-key",
            system_prompt="SYSTEM",
            user_prompt="元テキスト",
        )
        self.assertEqual(result["text"], "[自然に]整形済み")
        self.assertEqual(result["llm"]["provider"], "minimax")

    def test_preprocess_uses_azure_speech_purpose_mapping(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.openai_chat_json",
                return_value=("<speak></speak>", {"input_tokens": 9, "output_tokens": 18}),
            ) as chat_json,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="gpt-5.4-mini",
                api_key="openai-key",
                prompt_key="azure_speech.summary_preprocess",
            )

        chat_json.assert_called_once()
        self.assertEqual(result["text"], "<speak></speak>")
        self.assertEqual(result["llm"]["provider"], "openai")

    def test_preprocess_uses_anthropic_transport(self):
        service = TTSMarkupPreprocessService()
        message = type(
            "Message",
            (),
            {
                "content": [type("Content", (), {"text": "[自然に]整形済み"})()],
                "usage": type(
                    "Usage",
                    (),
                    {
                        "input_tokens": 7,
                        "output_tokens": 9,
                        "cache_creation_input_tokens": 0,
                        "cache_read_input_tokens": 0,
                    },
                )(),
            },
        )()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.anthropic_call_with_model_fallback",
                return_value=(message, "claude-sonnet-4-6", []),
            ) as anthropic_call,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="claude-sonnet-4-6",
                api_key="anthropic-key",
            )

        anthropic_call.assert_called_once_with(
            "SYSTEM\n\n元テキスト",
            "claude-sonnet-4-6",
            None,
            max_tokens=3200,
            api_key="anthropic-key",
            system_prompt="SYSTEM",
            user_prompt="元テキスト",
        )
        self.assertEqual(result["text"], "[自然に]整形済み")
        self.assertEqual(result["llm"]["provider"], "anthropic")

    def test_preprocess_uses_prompt_as_is_when_system_instruction_is_empty(self):
        service = TTSMarkupPreprocessService()
        message = type(
            "Message",
            (),
            {
                "content": [type("Content", (), {"text": "整形済み"})()],
                "usage": type(
                    "Usage",
                    (),
                    {
                        "input_tokens": 3,
                        "output_tokens": 4,
                        "cache_creation_input_tokens": 0,
                        "cache_read_input_tokens": 0,
                    },
                )(),
            },
        )()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "",
                    "prompt_text": "固定プロンプト\n## 【テキスト】",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.anthropic_call_with_model_fallback",
                return_value=(message, "claude-sonnet-4-6", []),
            ) as anthropic_call,
        ):
            service.preprocess(
                text="元テキスト",
                model="claude-sonnet-4-6",
                api_key="anthropic-key",
            )

        anthropic_call.assert_called_once_with(
            "固定プロンプト\n## 【テキスト】\n\n元テキスト",
            "claude-sonnet-4-6",
            None,
            max_tokens=3200,
            api_key="anthropic-key",
            system_prompt=None,
            user_prompt="固定プロンプト\n## 【テキスト】\n\n元テキスト",
        )

    def test_preprocess_enriches_elevenlabs_audio_briefing_single_prompt_variables(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{persona_name}}|{{tone_prompt}}|{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.resolve_audio_briefing_persona_prompts",
                return_value=("落ち着いていて整理がうまい", "", ""),
            ) as persona_prompts,
            patch(
                "app.services.tts_markup_preprocess.openrouter_chat_json",
                return_value=("[自然に]整形済み", {"input_tokens": 10, "output_tokens": 20}),
            ) as chat_json,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="openrouter::openai/gpt-5.4-mini",
                api_key="openrouter-key",
                prompt_key="elevenlabs.audio_briefing_single_preprocess",
                variables={"persona_name": "editor"},
            )

        persona_prompts.assert_called_once_with("editor")
        chat_json.assert_called_once_with(
            "editor|落ち着いていて整理がうまい|元テキスト",
            "openrouter::openai/gpt-5.4-mini",
            "openrouter-key",
            system_instruction="SYSTEM",
            max_output_tokens=3200,
        )
        self.assertEqual(result["llm"]["provider"], "openrouter")

    def test_preprocess_surfaces_anthropic_failure_detail(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "",
                    "prompt_text": "固定プロンプト\n## 【テキスト】",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.anthropic_call_with_model_fallback",
                return_value=(None, None, [{"model": "claude-haiku-4-5", "reason": "invalid_request_error"}]),
            ),
        ):
            with self.assertRaisesRegex(RuntimeError, "claude-haiku-4-5: invalid_request_error"):
                service.preprocess(
                    text="元テキスト",
                    model="claude-haiku-4-5",
                    api_key="anthropic-key",
                )

    def test_preprocess_defaults_prompt_key(self):
        service = TTSMarkupPreprocessService()

        with patch(
            "app.services.tts_markup_preprocess.get_default_prompt_template",
            return_value={"system_instruction": "SYSTEM", "prompt_text": "{{text}}"},
        ) as get_template, patch(
            "app.services.tts_markup_preprocess.openai_chat_json",
            return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
        ):
            service.preprocess(text="元テキスト", model="gpt-5.4-mini", api_key="openai-key")

        get_template.assert_called_once_with(DEFAULT_TTS_MARKUP_PREPROCESS_PROMPT_KEY)

    def test_preprocess_renders_supplied_variables(self):
        service = TTSMarkupPreprocessService()

        with patch(
            "app.services.tts_markup_preprocess.get_default_prompt_template",
            return_value={
                "system_instruction": "SYSTEM",
                "prompt_text": "話者: {{persona_name}}\n本文:\n{{text}}",
            },
        ), patch(
            "app.services.tts_markup_preprocess.openai_chat_json",
            return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
        ) as chat_json:
            service.preprocess(
                text="元テキスト",
                model="gpt-5.4-mini",
                api_key="openai-key",
                variables={"persona_name": "editor"},
            )

        chat_json.assert_called_once_with(
            "話者: editor\n本文:\n元テキスト",
            "gpt-5.4-mini",
            "openai-key",
            system_instruction="SYSTEM",
            max_output_tokens=3200,
        )

    def test_preprocess_enriches_audio_briefing_persona_tone_prompts(self):
        service = TTSMarkupPreprocessService()

        with patch(
            "app.services.tts_markup_preprocess.get_default_prompt_template",
            return_value={
                "system_instruction": "SYSTEM",
                "prompt_text": "話者: {{persona_name}}\nトーン: {{tone_prompt}}\n本文:\n{{text}}",
            },
        ), patch(
            "app.services.tts_markup_preprocess.openai_chat_json",
            return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
        ) as chat_json:
            service.preprocess(
                text="元テキスト",
                model="gpt-5.4-mini",
                api_key="openai-key",
                prompt_key="fish.audio_briefing_single_preprocess",
                variables={"persona_name": "editor"},
            )

        self.assertIn("話者: editor", chat_json.call_args.args[0])
        self.assertIn("落ち着いた編集者として、重要度と意味合いを端正に語る。", chat_json.call_args.args[0])

    def test_preprocess_enriches_audio_briefing_duo_persona_prompts(self):
        service = TTSMarkupPreprocessService()

        with patch(
            "app.services.tts_markup_preprocess.get_default_prompt_template",
            return_value={
                "system_instruction": "SYSTEM",
                "prompt_text": (
                    "ホスト: {{host_persona_name}} / {{host_tone_prompt}}\n"
                    "パートナー: {{partner_persona_name}} / {{partner_tone_prompt}}\n"
                    "{{text}}"
                ),
            },
        ), patch(
            "app.services.tts_markup_preprocess.openai_chat_json",
            return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
        ) as chat_json:
            service.preprocess(
                text="<|speaker:0|>冒頭<|speaker:1|>補足",
                model="gpt-5.4-mini",
                api_key="openai-key",
                prompt_key="fish.audio_briefing_duo_preprocess",
                variables={
                    "host_persona_name": "native",
                    "partner_persona_name": "analyst",
                },
            )

        rendered = chat_json.call_args.args[0]
        self.assertIn("ホスト: native / 明るく自然体のAIネイティブとして、体感と共有したくなる空気感を軽やかに語る。", rendered)
        self.assertIn("パートナー: analyst / 理知的に背景と含意を整理するアナリストとして、情報の筋道を丁寧に示す。", rendered)
        self.assertIn("<|speaker:0|>冒頭<|speaker:1|>補足", rendered)

    def test_preprocess_llm_meta_uses_separate_purpose(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={"system_instruction": "SYSTEM", "prompt_text": "{{text}}"},
            ),
            patch(
                "app.services.tts_markup_preprocess.openai_chat_json",
                return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
            ),
            patch("app.services.tts_markup_preprocess.openai_llm_meta", return_value={"purpose": FISH_PREPROCESS_PURPOSE}) as llm_meta,
        ):
            result = service.preprocess(text="元テキスト", model="gpt-5.4-mini", api_key="openai-key")

        llm_meta.assert_called_once_with("gpt-5.4-mini", FISH_PREPROCESS_PURPOSE, {"input_tokens": 1, "output_tokens": 1})
        self.assertEqual(result["llm"]["purpose"], FISH_PREPROCESS_PURPOSE)

    def test_preprocess_uses_gemini_tts_purpose_for_gemini_prompt_keys(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={"system_instruction": "SYSTEM", "prompt_text": "{{text}}"},
            ),
            patch(
                "app.services.tts_markup_preprocess.openai_chat_json",
                return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
            ),
            patch("app.services.tts_markup_preprocess.openai_llm_meta", return_value={"purpose": GEMINI_TTS_PREPROCESS_PURPOSE}) as llm_meta,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="gpt-5.4-mini",
                api_key="openai-key",
                prompt_key="gemini.summary_preprocess",
            )

        llm_meta.assert_called_once_with("gpt-5.4-mini", GEMINI_TTS_PREPROCESS_PURPOSE, {"input_tokens": 1, "output_tokens": 1})
        self.assertEqual(result["llm"]["purpose"], GEMINI_TTS_PREPROCESS_PURPOSE)

    def test_preprocess_uses_xai_tts_purpose_for_xai_prompt_keys(self):
        service = TTSMarkupPreprocessService()

        with (
            patch(
                "app.services.tts_markup_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.tts_markup_preprocess.xai_chat_json",
                return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
            ),
            patch("app.services.tts_markup_preprocess.xai_llm_meta", return_value={"purpose": XAI_TTS_PREPROCESS_PURPOSE}) as llm_meta,
        ):
            result = service.preprocess(
                text="元テキスト",
                model="grok-4-fast-non-reasoning",
                api_key="xai-key",
                prompt_key="xai.summary_preprocess",
            )

        llm_meta.assert_called_once_with("grok-4-fast-non-reasoning", XAI_TTS_PREPROCESS_PURPOSE, {"input_tokens": 1, "output_tokens": 1})
        self.assertEqual(result["llm"]["purpose"], XAI_TTS_PREPROCESS_PURPOSE)

    def test_prompt_family_routing_by_prompt_key(self):
        service = TTSMarkupPreprocessService()
        cases = [
            ("fish.summary_preprocess", FISH_PREPROCESS_PURPOSE),
            ("fish.audio_briefing_single_preprocess", FISH_PREPROCESS_PURPOSE),
            ("fish.audio_briefing_duo_preprocess", FISH_PREPROCESS_PURPOSE),
            ("gemini.summary_preprocess", GEMINI_TTS_PREPROCESS_PURPOSE),
            ("gemini.audio_briefing_single_preprocess", GEMINI_TTS_PREPROCESS_PURPOSE),
            ("gemini.audio_briefing_duo_preprocess", GEMINI_TTS_PREPROCESS_PURPOSE),
            ("elevenlabs.summary_preprocess", ELEVENLABS_TTS_PREPROCESS_PURPOSE),
            ("elevenlabs.audio_briefing_single_preprocess", ELEVENLABS_TTS_PREPROCESS_PURPOSE),
            ("elevenlabs.audio_briefing_duo_preprocess", ELEVENLABS_TTS_PREPROCESS_PURPOSE),
            ("xai.summary_preprocess", XAI_TTS_PREPROCESS_PURPOSE),
            ("xai.audio_briefing_single_preprocess", XAI_TTS_PREPROCESS_PURPOSE),
            ("xai.audio_briefing_duo_preprocess", XAI_TTS_PREPROCESS_PURPOSE),
        ]

        for prompt_key, want in cases:
            with self.subTest(prompt_key=prompt_key):
                self.assertEqual(service._purpose_for_prompt_key(prompt_key), want)

    def test_prompt_family_routing_rejects_unknown_prompt_key(self):
        service = TTSMarkupPreprocessService()

        with self.assertRaisesRegex(RuntimeError, "unsupported tts markup preprocess prompt key"):
            service._purpose_for_prompt_key("custom.summary_preprocess")


if __name__ == "__main__":
    unittest.main()
