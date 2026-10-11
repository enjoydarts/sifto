import base64
import copy
import unittest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
try:
    from maintenance import plan as p
except ImportError:
    p = None


# Synthetic Ubuntu apt LC_ALL=C records: existing, newly installed, and multiarch.
SIMULATION = """Reading package lists...
Building dependency tree...
Inst openssl [3.0.13-0ubuntu3] (3.0.13-0ubuntu3.5 Ubuntu:24.04/noble-updates [arm64])
Inst libssl3t64:arm64 [3.0.13-0ubuntu3] (3.0.13-0ubuntu3.5 Ubuntu:24.04/noble-updates [arm64])
Inst linux-modules-6.8.0-60-generic (6.8.0-60.63 Ubuntu:24.04/noble-updates [arm64])
Conf openssl (3.0.13-0ubuntu3.5 Ubuntu:24.04/noble-updates [arm64])
"""

# Only generic OS records retained from an actual Ubuntu 24.04 ARM64 apt
# simulation captured 2026-10-11. Host inventory is kept private.
CAPTURED_UBUNTU_FRAGMENT = """Inst motd-news-config [13ubuntu10.4] (13ubuntu10.5 Ubuntu:24.04/noble-updates [all])
Inst base-files [13ubuntu10.4] (13ubuntu10.5 Ubuntu:24.04/noble-updates [arm64])
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

    def test_parser_accepts_captured_ubuntu_release_version_records(self):
        result = p.parse_simulation(CAPTURED_UBUNTU_FRAGMENT)
        self.assertEqual(result, [{"name": "base-files", "installed": "13ubuntu10.4", "target": "13ubuntu10.5"},
                                  {"name": "motd-news-config", "installed": "13ubuntu10.4", "target": "13ubuntu10.5"}])

    def test_removal_and_malformed_inst_are_rejected(self):
        for output in [SIMULATION + "Remv openssl [3.0.13]\n", "Inst evil;touch /tmp/pwn (1 repo [arm64])",
                       "Inst openssl [1] missing-version", "Inst x (1 repo)\nInst x (2 repo)"]:
            with self.subTest(output=output), self.assertRaises(p.MaintenanceError):
                p.parse_simulation(output)

    def test_build_binds_current_state_and_limits_lifetime(self):
        plan = self.candidate()
        self.assertEqual(plan["state_sha256"], p.state_sha256(inventory()))
        self.assertEqual(plan["packages"], p.parse_simulation(SIMULATION, canonical_architecture=True))
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

    def test_unavailable_simulation_is_unknown_and_cannot_build_upgrade_plan(self):
        inv = inventory()
        inv["simulation_available"] = False
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, self.sha, now=1000)
        self.assertEqual(p.build_plan(inv, self.sha, action="reboot", allow_reboot=True, now=1000)["packages"], [])

    def test_nonempty_apt_plan_requires_explicit_package_script_restart_scope(self):
        plan = self.candidate()
        self.assertIn("restart_scope", plan)
        self.assertEqual(plan["restart_scope"], "package_scripts_and_listed_services")
        plan["restart_scope"] = "listed_services_only"
        with self.assertRaises(p.MaintenanceError):
            p.validate_plan(plan)

    def critical_inventory(self, name, policy):
        inv = inventory()
        inv["packages"].append({"name": name, "version": "1.0.0"})
        inv["simulation"] = f"Inst {name} [1.0.0] (2.0.0 Ubuntu:24.04/noble [arm64])\n"
        inv["maintenance_policy"] = policy
        return inv

    def test_runtime_packages_require_explicit_docker_and_configured_containerd_scope(self):
        for name in ["docker-ce", "docker.io", "moby-engine", "containerd", "containerd.io"]:
            inv = self.critical_inventory(name, {"allowed_restart_services": ["docker.service", "containerd.service"]})
            inv["packages"][-1]["version"] = "1.0.0"
            inv["simulation"] = f"Inst {name} [1.0.0] (2.0.0 Ubuntu:24.04/noble [arm64])\n"
            with self.subTest(name=name):
                self.assertEqual(p.build_plan(inv, self.sha, now=1000)["restart_services"], ["containerd.service", "docker.service"])

    def test_critical_runtime_update_declines_without_root_restart_whitelist(self):
        inv = self.critical_inventory("containerd.io", {"allowed_restart_services": []})
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, self.sha, now=1000)

    def test_postgres_common_requires_configured_cluster_restart_scope(self):
        inv = self.critical_inventory("postgresql-common", {"allowed_restart_services": ["postgresql@16-main.service"],
                                                          "database_restart_services": ["postgresql@16-main.service"]})
        self.assertEqual(p.build_plan(inv, self.sha, now=1000)["restart_services"], ["postgresql@16-main.service"])
        inv["maintenance_policy"]["database_restart_services"] = []
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, self.sha, now=1000)

    def test_database_library_upgrade_lists_configured_postgres_cluster(self):
        inv = inventory()
        inv["maintenance_policy"] = {"host_role": "database", "allowed_restart_services": ["postgresql@16-main.service"],
                                     "database_restart_services": ["postgresql@16-main.service"]}
        self.assertEqual(p.build_plan(inv, self.sha, now=1000)["restart_services"], ["postgresql@16-main.service"])

    def test_pooler_packages_require_their_own_whitelisted_service(self):
        for name in ["pgbouncer", "pgpool2"]:
            inv = self.critical_inventory(name, {"allowed_restart_services": [name + ".service"]})
            with self.subTest(name=name):
                self.assertEqual(p.build_plan(inv, self.sha, now=1000)["restart_services"], [name + ".service"])

    def test_containerd_package_cannot_decline_its_restart_by_omitting_whitelist_unit(self):
        inv = self.critical_inventory("containerd.io", {"allowed_restart_services": ["docker.service"]})
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, self.sha, now=1000)

    def test_unapproved_conf_only_package_is_rejected(self):
        with self.assertRaises(p.MaintenanceError):
            p.parse_simulation(SIMULATION + "Conf pending (1.0 Ubuntu:24.04/noble [arm64])\n")

    def test_conf_target_must_match_its_install_record(self):
        with self.assertRaises(p.MaintenanceError):
            p.parse_simulation("Inst example (1.0 Ubuntu:24.04/noble [arm64])\nConf example (2.0 Ubuntu:24.04/noble [arm64])\n")

    def test_package_architecture_cannot_disagree_with_apt_target_metadata(self):
        inv = inventory()
        inv["simulation"] = "Inst example:amd64 (1.0 Ubuntu:24.04/noble [arm64])\n"
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, self.sha, now=1000)


if __name__ == "__main__":
    unittest.main()
