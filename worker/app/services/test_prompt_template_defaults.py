import unittest

from app.services.prompt_template_defaults import get_default_prompt_template, render_prompt_template


class PromptTemplateDefaultsTest(unittest.TestCase):
    def test_render_prompt_template_does_not_reexpand_inserted_braces(self):
        rendered = render_prompt_template(
            "Title: {{title}}\nFacts: {{facts}}",
            {
                "title": "literal {{facts}}",
                "facts": "{json_like}",
            },
        )

        self.assertEqual(rendered, "Title: literal {{facts}}\nFacts: {json_like}")

    def test_get_default_prompt_template_loads_elevenlabs_templates(self):
        for prompt_key in [
            "elevenlabs.summary_preprocess",
            "elevenlabs.audio_briefing_single_preprocess",
            "elevenlabs.audio_briefing_duo_preprocess",
        ]:
            template = get_default_prompt_template(prompt_key)
            self.assertEqual(template["prompt_key"], prompt_key)
            self.assertIn("prompt_text", template)
            self.assertTrue(str(template["prompt_text"]).strip())


if __name__ == "__main__":
    unittest.main()
