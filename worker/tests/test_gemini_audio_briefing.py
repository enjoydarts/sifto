import os
import unittest
from unittest.mock import patch

from app.services.feed_task_common import AUDIO_BRIEFING_CHARS_PER_MINUTE
from app.services.gemini_service import generate_audio_briefing_script


class GeminiAudioBriefingContextCacheTests(unittest.TestCase):
    def test_generate_audio_briefing_script_passes_context_cache_key(self):
        captured = {}

        def fake_generate_content(prompt, **kwargs):
            captured["prompt"] = prompt
            captured["kwargs"] = kwargs
            return (
                '{"opening":"導入です。","overall_summary":"総括です。","article_segments":[],"ending":"締めです。"}',
                {"input_tokens": 10, "output_tokens": 20, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0},
            )

        with patch.dict(os.environ, {"GEMINI_AUDIO_BRIEFING_SCRIPT_CONTEXT_CACHE": "1"}, clear=False), patch(
            "app.services.gemini_service._cache_key_hash", return_value="cache-key"
        ), patch("app.services.gemini_service._generate_content", side_effect=fake_generate_content):
            generate_audio_briefing_script(
                persona="editor",
                articles=[],
                intro_context={},
                target_duration_minutes=20,
                target_chars=14000,
                chars_per_minute=AUDIO_BRIEFING_CHARS_PER_MINUTE,
                include_opening=True,
                include_overall_summary=True,
                include_article_segments=False,
                include_ending=True,
                api_key="test-key",
                model="gemini-2.5-flash",
            )

        self.assertEqual(captured["kwargs"]["context_cache_key"], "cache-key")
        self.assertIn("persona: editor", captured["kwargs"]["system_instruction"])
        self.assertIn("articles:", captured["prompt"])


if __name__ == "__main__":
    unittest.main()
