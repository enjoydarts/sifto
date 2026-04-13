import unittest

from app.services.facts_task_common import build_facts_task
from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import bind_prompt_override


class FactsTaskCommonTests(unittest.TestCase):
    def test_default_template_override_matches_code_default_rendering(self):
        expected = build_facts_task(
            "OpenAI updates model lineup",
            "OpenAI announced a new lineup for enterprise customers.",
        )
        default_template = get_default_prompt_template("facts.default")

        with bind_prompt_override("facts.default", default_template["prompt_text"], default_template["system_instruction"]):
            actual = build_facts_task(
                "OpenAI updates model lineup",
                "OpenAI announced a new lineup for enterprise customers.",
            )

        self.assertEqual(actual["system_instruction"], expected["system_instruction"])
        self.assertEqual(actual["prompt"], expected["prompt"])


if __name__ == "__main__":
    unittest.main()
