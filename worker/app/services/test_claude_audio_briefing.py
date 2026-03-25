import unittest
from types import SimpleNamespace
from unittest.mock import patch

from app.services.claude_service import generate_audio_briefing_script


class ClaudeAudioBriefingPromptCacheTests(unittest.TestCase):
    def test_generate_audio_briefing_script_enables_prompt_cache_with_split_prompts(self):
        fake_message = SimpleNamespace(
            content=[SimpleNamespace(text='{"opening":"導入です。","overall_summary":"総括です。","article_segments":[],"ending":"締めです。"}')],
            usage=SimpleNamespace(input_tokens=10, output_tokens=20, cache_creation_input_tokens=0, cache_read_input_tokens=0),
        )
        captured = {}

        def fake_call(prompt, primary_model, fallback_model, **kwargs):
            captured["prompt"] = prompt
            captured["primary_model"] = primary_model
            captured["fallback_model"] = fallback_model
            captured["kwargs"] = kwargs
            return fake_message, primary_model, []

        with patch("app.services.claude_service._client_for_api_key", return_value=object()), patch(
            "app.services.claude_service._call_with_model_fallback", side_effect=fake_call
        ):
            generate_audio_briefing_script(
                persona="editor",
                articles=[],
                intro_context={},
                target_duration_minutes=20,
                target_chars=14000,
                chars_per_minute=700,
                include_opening=True,
                include_overall_summary=True,
                include_article_segments=False,
                include_ending=True,
                api_key="test-key",
                model="claude-sonnet-4-5",
            )

        self.assertTrue(captured["kwargs"]["enable_prompt_cache"])
        self.assertIn("persona: editor", captured["kwargs"]["system_prompt"])
        self.assertIn("articles:", captured["kwargs"]["user_prompt"])
        self.assertEqual(captured["prompt"], captured["kwargs"]["user_prompt"])


if __name__ == "__main__":
    unittest.main()
