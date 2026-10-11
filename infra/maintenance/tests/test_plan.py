import base64
import copy
import unittest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
try:
    from maintenance import plan as p
except ImportError:
    p = None


# Ubuntu apt LC_ALL=C simulation output: existing, newly installed, and multiarch.
SIMULATION = """Reading package lists...
Building dependency tree...
Inst openssl [3.0.13-0ubuntu3] (3.0.13-0ubuntu3.5 Ubuntu:24.04/noble-updates [arm64])
Inst libssl3t64:arm64 [3.0.13-0ubuntu3] (3.0.13-0ubuntu3.5 Ubuntu:24.04/noble-updates [arm64])
Inst linux-modules-6.8.0-60-generic (6.8.0-60.63 Ubuntu:24.04/noble-updates [arm64])
Conf openssl (3.0.13-0ubuntu3.5 Ubuntu:24.04/noble-updates [arm64])
"""


def inventory():
    return {"schema_version": 1, "host_id": "sifto-app", "platform": {
        "id": "ubuntu", "version": "24.04", "architecture": "arm64"},
        "packages": [{"name": "openssl", "version": "3.0.13-0ubuntu3"},
                     {"name": "libssl3t64:arm64", "version": "3.0.13-0ubuntu3"}],
        "runtime": {"boot_id": "boot-a", "running_kernel": "6.8.0-59-generic",
                    "docker_version": "28.0.0", "containers": []},
        "simulation": SIMULATION, "collected_at": 1000}


class PlanTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(p, "signed-plan implementation is missing")
        self.key = Ed25519PrivateKey.generate()
        self.sha = "a" * 40

    def candidate(self):
        return p.build_plan(inventory(), self.sha, now=1000)

    def envelope(self):
        plan = self.candidate()
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123",
                    "approved_by": "reviewer", "approved_at": 1001,
                    "expires_at": 2000, "workflow_sha": self.sha}
        return p.sign_plan(plan, approval, self.key)

    def test_parser_captures_dependency_closure_and_multiarch(self):
        result = p.parse_simulation(SIMULATION)
        self.assertEqual(len(result), 3)
        self.assertEqual(result[-1], {"name": "openssl", "installed": "3.0.13-0ubuntu3",
                                    "target": "3.0.13-0ubuntu3.5"})
        self.assertIsNone(next(x for x in result if x["name"].startswith("linux-modules"))["installed"])

    def test_removal_and_malformed_inst_are_rejected(self):
        for output in [SIMULATION + "Remv openssl [3.0.13]\n", "Inst evil;touch /tmp/pwn (1 repo [arm64])",
                       "Inst openssl [1] missing-version", "Inst x (1 repo)\nInst x (2 repo)"]:
            with self.subTest(output=output), self.assertRaises(p.MaintenanceError):
                p.parse_simulation(output)

    def test_build_binds_current_state_and_limits_lifetime(self):
        plan = self.candidate()
        self.assertEqual(plan["state_sha256"], p.state_sha256(inventory()))
        self.assertEqual(plan["packages"], p.parse_simulation(SIMULATION))
        self.assertLessEqual(plan["expires_at"] - plan["created_at"], 86400)
        changed = inventory()
        changed["collected_at"] = 9999
        changed["runtime"]["containers"] = []
        self.assertEqual(p.state_sha256(changed), plan["state_sha256"])
        changed["runtime"]["boot_id"] = "boot-b"
        self.assertNotEqual(p.state_sha256(changed), plan["state_sha256"])

    def test_signature_round_trip_and_tampering_rejected(self):
        signed = self.envelope()
        self.assertEqual(p.verify_envelope(signed, self.key.public_key(), "sifto-app", self.sha, now=1010), signed["plan"])
        tampered = copy.deepcopy(signed)
        tampered["plan"]["allow_reboot"] = True
        with self.assertRaises(p.MaintenanceError):
            p.verify_envelope(tampered, self.key.public_key(), "sifto-app", self.sha, now=1010)

    def test_wrong_host_workflow_and_expiry_rejected(self):
        signed = self.envelope()
        for host, sha, now in [("other", self.sha, 1010), ("sifto-app", "b" * 40, 1010), ("sifto-app", self.sha, 2000)]:
            with self.subTest(host=host, sha=sha, now=now), self.assertRaises(p.MaintenanceError):
                p.verify_envelope(signed, self.key.public_key(), host, sha, now=now)

    def test_invalid_plan_data_is_rejected_before_signing(self):
        for field, value in [("workflow_sha", "main"), ("expires_at", 100000),
                             ("allow_service_restarts", "yes"),
                             ("packages", [{"name": "openssl;id", "installed": "1", "target": "2"}]),
                             ("restart_services", ["docker;id.service"])]:
            plan = self.candidate()
            plan[field] = value
            with self.subTest(field=field), self.assertRaises(p.MaintenanceError):
                p.validate_plan(plan)

    def test_unknown_fields_and_nonfinite_json_are_rejected(self):
        with self.assertRaises(p.MaintenanceError):
            p.canonical_json({"x": float("nan")})
        plan = self.candidate()
        plan["shell"] = "id"
        with self.assertRaises(p.MaintenanceError):
            p.validate_plan(plan)


if __name__ == "__main__":
    unittest.main()
