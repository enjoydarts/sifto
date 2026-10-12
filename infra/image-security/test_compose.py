"""Check rendered Compose identities against the command's actual caller."""
import json
import os
from pathlib import Path
import unittest


class ComposeCallerIdentity(unittest.TestCase):
    def test_helpers_run_as_caller_and_keep_archive_read_only(self):
        uid, gid = os.environ["IMAGE_SECURITY_UID"], os.environ["IMAGE_SECURITY_GID"]
        self.assertEqual(os.getuid(), int(uid))
        self.assertEqual(os.getgid(), int(gid))
        config = json.loads(Path(os.environ.get("IMAGE_SECURITY_COMPOSE_CONFIG", "/reports/compose.json")).read_text())
        for name in ("verifier", "trivy"):
            service = config["services"][name]
            self.assertEqual(service.get("user"), f"{uid}:{gid}")
            self.assertEqual(service["cap_drop"], ["ALL"])
            self.assertNotIn("cap_add", service)
            self.assertTrue(service["read_only"])
            self.assertTrue(any(mount["target"] == "/reports/image.tar" and mount["read_only"]
                                for mount in service["volumes"]))
        cache = next(mount for mount in config["services"]["trivy"]["volumes"] if mount["target"] == "/cache")
        self.assertEqual(cache["type"], "bind", "Cache must be created by the caller, not a root-owned Docker volume")


if __name__ == "__main__":
    unittest.main()
