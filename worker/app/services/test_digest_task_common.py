import unittest

from app.services.digest_task_common import DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS, build_digest_task
from app.services.prompt_template_defaults import get_default_prompt_template
from app.services.runtime_prompt_overrides import bind_prompt_override


class DigestTaskCommonTests(unittest.TestCase):
    def test_digest_cluster_draft_max_output_tokens(self):
        self.assertEqual(DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS, 2500)

    def test_default_template_override_matches_code_default_rendering(self):
        expected = build_digest_task(
            "2026-04-01",
            2,
            "- item=1 rank=1 | title=OpenAI updates model lineup | topics=AI | score=0.9 | summary=Summary\n"
            "- item=2 rank=2 | title=Google announces change | topics=Search | score=0.8 | summary=Summary",
        )
        default_template = get_default_prompt_template("digest.default")

        with bind_prompt_override("digest.default", default_template["prompt_text"], default_template["system_instruction"]):
            actual = build_digest_task(
                "2026-04-01",
                2,
                "- item=1 rank=1 | title=OpenAI updates model lineup | topics=AI | score=0.9 | summary=Summary\n"
                "- item=2 rank=2 | title=Google announces change | topics=Search | score=0.8 | summary=Summary",
            )

        self.assertEqual(actual["system_instruction"], expected["system_instruction"])
        self.assertEqual(actual["prompt"], expected["prompt"])


if __name__ == "__main__":
    unittest.main()
