import unittest

from app.services.runtime_prompt_overrides import apply_prompt_override, bind_prompt_override


class RuntimePromptOverridesTest(unittest.TestCase):
    def test_apply_prompt_override_respects_explicit_empty_values(self):
        with bind_prompt_override("summary.default", "", ""):
            system_instruction, prompt_text = apply_prompt_override(
                "summary.default",
                "default system",
                "default prompt",
                {"title": "Example"},
            )

        self.assertEqual(system_instruction, "")
        self.assertEqual(prompt_text, "")


if __name__ == "__main__":
    unittest.main()
