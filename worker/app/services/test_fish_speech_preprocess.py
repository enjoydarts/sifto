import unittest
from unittest.mock import patch

from app.services.fish_speech_preprocess import (
    DEFAULT_FISH_PREPROCESS_PROMPT_KEY,
    FISH_PREPROCESS_PURPOSE,
    FishSpeechPreprocessService,
)


class FishSpeechPreprocessServiceTests(unittest.TestCase):
    def test_preprocess_uses_openai_compatible_transport(self):
        service = FishSpeechPreprocessService()

        with (
            patch(
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.fish_speech_preprocess.openai_chat_json",
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
        service = FishSpeechPreprocessService()

        with (
            patch(
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "固定プロンプト\n## 【テキスト】",
                },
            ),
            patch(
                "app.services.fish_speech_preprocess.openai_chat_json",
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
        service = FishSpeechPreprocessService()

        with (
            patch(
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.fish_speech_preprocess.gemini_generate_content",
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

    def test_preprocess_uses_anthropic_transport(self):
        service = FishSpeechPreprocessService()
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
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "SYSTEM",
                    "prompt_text": "{{text}}",
                },
            ),
            patch(
                "app.services.fish_speech_preprocess.anthropic_call_with_model_fallback",
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
        service = FishSpeechPreprocessService()
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
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "",
                    "prompt_text": "固定プロンプト\n## 【テキスト】",
                },
            ),
            patch(
                "app.services.fish_speech_preprocess.anthropic_call_with_model_fallback",
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

    def test_preprocess_surfaces_anthropic_failure_detail(self):
        service = FishSpeechPreprocessService()

        with (
            patch(
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={
                    "system_instruction": "",
                    "prompt_text": "固定プロンプト\n## 【テキスト】",
                },
            ),
            patch(
                "app.services.fish_speech_preprocess.anthropic_call_with_model_fallback",
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
        service = FishSpeechPreprocessService()

        with patch(
            "app.services.fish_speech_preprocess.get_default_prompt_template",
            return_value={"system_instruction": "SYSTEM", "prompt_text": "{{text}}"},
        ) as get_template, patch(
            "app.services.fish_speech_preprocess.openai_chat_json",
            return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
        ):
            service.preprocess(text="元テキスト", model="gpt-5.4-mini", api_key="openai-key")

        get_template.assert_called_once_with(DEFAULT_FISH_PREPROCESS_PROMPT_KEY)

    def test_preprocess_llm_meta_uses_separate_purpose(self):
        service = FishSpeechPreprocessService()

        with (
            patch(
                "app.services.fish_speech_preprocess.get_default_prompt_template",
                return_value={"system_instruction": "SYSTEM", "prompt_text": "{{text}}"},
            ),
            patch(
                "app.services.fish_speech_preprocess.openai_chat_json",
                return_value=("整形済み", {"input_tokens": 1, "output_tokens": 1}),
            ),
            patch("app.services.fish_speech_preprocess.openai_llm_meta", return_value={"purpose": FISH_PREPROCESS_PURPOSE}) as llm_meta,
        ):
            result = service.preprocess(text="元テキスト", model="gpt-5.4-mini", api_key="openai-key")

        llm_meta.assert_called_once_with("gpt-5.4-mini", FISH_PREPROCESS_PURPOSE, {"input_tokens": 1, "output_tokens": 1})
        self.assertEqual(result["llm"]["purpose"], FISH_PREPROCESS_PURPOSE)


if __name__ == "__main__":
    unittest.main()
