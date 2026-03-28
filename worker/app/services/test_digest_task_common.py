import unittest

from app.services.digest_task_common import DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS


class DigestTaskCommonTests(unittest.TestCase):
    def test_digest_cluster_draft_max_output_tokens(self):
        self.assertEqual(DIGEST_CLUSTER_DRAFT_MAX_OUTPUT_TOKENS, 2500)


if __name__ == "__main__":
    unittest.main()
