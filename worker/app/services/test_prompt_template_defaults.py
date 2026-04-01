import unittest

from app.services.prompt_template_defaults import render_prompt_template


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


if __name__ == "__main__":
    unittest.main()
