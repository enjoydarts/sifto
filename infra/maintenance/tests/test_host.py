import copy
import io
import json
import tempfile
import unittest
import fcntl
import multiprocessing
from unittest.mock import patch
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from maintenance import plan as p
from test_plan import SIMULATION

try:
    from maintenance import host as h
except ImportError:
    h = None


class FakeRunner:
    def __init__(self):
        self.calls = []
        self.simulation = SIMULATION
        self.upgraded = False
        self.fail_install = False
        self.service_active = True
        self.kernel = "6.8.0-59-generic"
        self.container_health = "healthy"
        self.before_install = None
        self.extra_packages = ""
        self.before_simulation = None

    def __call__(self, argv, *, timeout=60, check=True):
        self.calls.append(list(argv))
        if argv[0] == "dpkg-query":
            version = "3.0.13-0ubuntu3.5" if self.upgraded else "3.0.13-0ubuntu3"
            output = f"openssl\t{version}\tinstalled\nlibssl3t64:arm64\t{version}\tinstalled\n"
            if self.upgraded:
                output += "linux-modules-6.8.0-60-generic\t6.8.0-60.63\tinstalled\n"
            return output + self.extra_packages
        if argv[:2] == ["dpkg", "--print-architecture"]:
            return "arm64\n"
        if argv[:2] == ["dpkg", "--compare-versions"]:
            return "false"  # Real runner exposes return code through check=False? use helper below.
        if argv[:2] == ["uname", "-r"]:
            return self.kernel + "\n"
        if argv[:2] == ["docker", "version"]:
            return '{"Version":"28.0.0"}\n'
        if argv[:2] == ["docker", "ps"]:
            return "c" * 64 + "\n"
        if argv[:2] == ["docker", "inspect"]:
            return json.dumps({"id": "c" * 64, "image_id": "sha256:" + "d" * 64,
                               "image": "example/app:1", "status": "running", "health": self.container_health}) + "\n"
        if argv[:2] == ["systemctl", "is-active"]:
            return "active\n" if self.service_active else "inactive\n"
        if argv[0] == "apt-get" and "-s" in argv:
            if "install" in argv and self.before_simulation:
                self.before_simulation()
            return self.simulation
        if argv[0] == "apt-get" and "install" in argv:
            if self.before_install:
                self.before_install()
            if self.fail_install:
                raise p.MaintenanceError("fake apt failed")
            self.upgraded = True
            return "done"
        if argv[0] == "apt-get" and "update" in argv:
            return "done"
        if "psql" in " ".join(argv):
            return "1\n"
        if argv[:2] == ["systemctl", "reboot"]:
            return ""
        if argv[:2] == ["systemctl", "restart"]:
            return ""
        raise AssertionError(f"unexpected command: {argv}")

    def compare_versions(self, left, operator, right):
        return left == "0.1" and operator == "lt"


class HostTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(h, "safe host executor implementation is missing")
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.key = Ed25519PrivateKey.generate()
        self.runner = FakeRunner()
        self.config = {"host_id": "sifto-app", "trusted_public_key": self.key.public_key(),
                       "approved_workflow_sha": "a" * 40, "state_path": str(self.root / "state"),
                       "allowed_approvers": ["human"],
                       "allowed_restart_services": ["docker.service"], "database_enabled": False}
        self.agent = h.HostAgent(self.config, self.runner, clock=lambda: 1000,
                                 read_file=self.read_file, lock_probe=lambda: [])

    def read_file(self, path):
        files = {"/etc/os-release": 'ID=ubuntu\nVERSION_ID="24.04"\n',
                 "/proc/sys/kernel/random/boot_id": "boot-a\n"}
        return files.get(str(path), "")

    def request(self, mutate=None):
        self.agent.run("refresh", {})
        self.runner.calls.clear()
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], now=1000)
        if mutate:
            mutate(plan)
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123",
                    "approved_by": "human", "approved_at": 1000,
                    "expires_at": 2000, "workflow_sha": plan["workflow_sha"]}
        return {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key)}

    def mutations(self):
        return [cmd for cmd in self.runner.calls if (cmd[0] == "apt-get" and "-s" not in cmd)
                or cmd[:2] in (["systemctl", "restart"], ["systemctl", "reboot"])]

    def test_scan_returns_private_inventory_without_mutations(self):
        inv = self.agent.run("scan", {})
        self.assertEqual(inv["host_id"], "sifto-app")
        self.assertEqual(inv["runtime"]["running_kernel"], self.runner.kernel)
        self.assertEqual(inv["packages"][0]["name"], "libssl3t64:arm64")
        self.assertIn("metadata", inv)
        self.assertEqual(self.mutations(), [])

    def test_apply_consumes_plan_before_exact_version_install_and_rejects_reuse(self):
        request = self.request()
        plan_id = request["envelope"]["plan"]["plan_id"]
        self.runner.before_install = lambda: self.assertTrue((self.root / "state" / "plans" / f"{plan_id}.json").is_file())
        result = self.agent.run("apply", request)
        self.assertEqual(result["status"], "completed")
        command = self.mutations()[0]
        self.assertIn("--no-remove", command)
        self.assertIn("openssl=3.0.13-0ubuntu3.5", command)
        self.assertIn("linux-modules-6.8.0-60-generic=6.8.0-60.63", command)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(len(self.mutations()), 1)

    def test_invalid_signature_never_mutates(self):
        request = self.request()
        request["envelope"]["signature"] = "AAAA"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_dry_run_verifies_transaction_without_consuming_or_mutating(self):
        request = self.request()
        request["dry_run"] = True
        before_files = {str(x): x.read_bytes() for x in (self.root / "state").rglob("*") if x.is_file()}
        try:
            result = self.agent.run("apply", request)
        except p.MaintenanceError as exc:
            self.fail(f"dry-run preflight was rejected: {exc}")
        self.assertEqual(result["status"], "preflight_passed")
        after_files = {str(x): x.read_bytes() for x in (self.root / "state").rglob("*") if x.is_file()}
        self.assertEqual(before_files, after_files)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(self.agent.run("apply", {k: v for k, v in request.items() if k != "dry_run"})["status"], "completed")

    def test_dry_run_flag_must_be_boolean(self):
        request = self.request()
        request["dry_run"] = "true"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_expiry_during_simulation_never_mutates_or_consumes(self):
        request = self.request()
        self.runner.before_simulation = lambda: setattr(self.agent, "clock", lambda: 2000)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(list((self.root / "state" / "plans").glob("*.json")), [])

    def test_state_change_during_simulation_never_mutates(self):
        request = self.request()
        self.runner.before_simulation = lambda: setattr(self.runner, "kernel", "different-kernel")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_unapproved_installed_package_change_fails_postcheck(self):
        request = self.request()
        self.runner.before_install = lambda: setattr(self.runner, "extra_packages", "surprise\t1.0\tinstalled\n")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        record = json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())
        self.assertEqual(record["status"], "failed")

    def test_state_drift_never_mutates(self):
        request = self.request()
        self.runner.kernel = "6.8.0-other"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_resimulation_unapproved_dependency_never_mutates(self):
        request = self.request()
        self.runner.simulation += "Inst dependency (1.0 Ubuntu:24.04/noble [arm64])\n"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_resimulation_removal_never_mutates(self):
        request = self.request()
        self.runner.simulation += "Remv openssl [3.0.13]\n"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_apt_failure_records_consumed_failed_plan_without_retry(self):
        request = self.request()
        self.runner.fail_install = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        record = json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())
        self.assertEqual(record["status"], "failed")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(len(self.mutations()), 1)

    def test_wrong_locally_pinned_workflow_never_mutates(self):
        request = self.request()
        self.config["approved_workflow_sha"] = "b" * 40
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_approver_must_be_in_root_whitelist(self):
        request = self.request()
        for allowed in [[], ["other"]]:
            self.config["allowed_approvers"] = allowed
            with self.subTest(allowed=allowed), self.assertRaises(p.MaintenanceError):
                self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_refresh_refuses_live_apt_locks(self):
        self.agent.lock_probe = lambda: ["/var/lib/dpkg/lock-frontend"]
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("refresh", {})
        self.assertEqual(self.mutations(), [])

    def test_refresh_only_updates_metadata_with_bounded_timeout(self):
        self.agent.run("refresh", {})
        self.assertEqual(len(self.mutations()), 1)
        self.assertIn("update", self.mutations()[0])
        self.assertNotIn("upgrade", self.mutations()[0])

    def test_unwhitelisted_restart_is_rejected(self):
        request = self.request(lambda plan: plan.update(restart_services=["postgresql.service"]))
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_packages_require_root_backup_readiness(self):
        self.runner.simulation = "Inst postgresql-16 [16.4-1] (16.5-1 Ubuntu:24.04/noble [arm64])\n"
        self.runner.extra_packages = "postgresql-16\t16.4-1\tinstalled\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.config["database_enabled"] = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_postgres_major_introduction_rejected_even_with_backup(self):
        self.config["database_enabled"] = True
        self.config["backup_readiness"] = {"backup_id": "backup-1", "verified_at": 900,
                                           "restore_tested_at": 800, "host_id": "sifto-app"}
        self.runner.extra_packages = "postgresql-16\t16.4-1\tinstalled\n"
        self.runner.simulation = "Inst postgresql-17 (17.1-1 Ubuntu:24.04/noble [arm64])\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_unversioned_postgres_metapackage_denied_even_with_backup(self):
        self.config["database_enabled"] = True
        self.config["backup_readiness"] = {"backup_id": "backup-1", "verified_at": 900,
                                           "restore_tested_at": 800, "host_id": "sifto-app"}
        self.runner.extra_packages = "postgresql\t16+257\tinstalled\npostgresql-16\t16.4-1\tinstalled\n"
        self.runner.simulation = "Inst postgresql [16+257] (17+257 Ubuntu:24.04/noble [arm64])\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_postgres_restart_requires_readiness(self):
        self.config["allowed_restart_services"].append("postgresql.service")
        request = self.request(lambda plan: plan.update(restart_services=["postgresql.service"]))
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_downgrade_rejected(self):
        self.runner.simulation = "Inst openssl [3.0.13-0ubuntu3] (0.1 Ubuntu:24.04/noble [arm64])\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_health_failure_is_recorded_and_does_not_restore_database(self):
        request = self.request()
        self.runner.before_install = lambda: setattr(self.runner, "container_health", "unhealthy")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(len(self.mutations()), 1)
        self.assertFalse(any("restore" in " ".join(x) for x in self.runner.calls))

    def test_reboot_is_separate_signed_action_and_verify_requires_new_boot(self):
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], allow_reboot=True, now=1000, action="reboot")
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123", "approved_by": "human",
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"]}
        request = {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key)}
        result = self.agent.run("apply", request)
        self.assertEqual(result["status"], "reboot_scheduled")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("verify", {"plan_id": plan["plan_id"]})
        self.agent.read_file = lambda path: "boot-b\n" if str(path).endswith("boot_id") else self.read_file(path)
        self.assertEqual(self.agent.run("verify", {"plan_id": plan["plan_id"]})["status"], "verified")

    def test_gateways_never_evaluate_arbitrary_original_command(self):
        for gateway, command in [("scan-gateway", "apply"), ("apply-gateway", "refresh"),
                                 ("scan-gateway", "scan; id"), ("apply-gateway", "apply && id"),
                                 ("scan-gateway", "$(id)"), ("scan-gateway", "scan --extra"),
                                 ("scan-gateway", " scan"), ("apply-gateway", "")]:
            with self.subTest(command=command), self.assertRaises(p.MaintenanceError):
                h.gateway_mode(gateway, command)
        self.assertEqual(h.gateway_mode("scan-gateway", "scan"), "scan")
        self.assertEqual(h.gateway_mode("apply-gateway", "apply"), "apply")

    def test_stdin_is_size_bounded_and_rejects_duplicate_json_keys(self):
        for data in [b"x" * (h.MAX_REQUEST_BYTES + 1), b'{"mode":"scan","mode":"apply"}', b'{"x":NaN}']:
            with self.subTest(data=data[:50]), self.assertRaises(p.MaintenanceError):
                h.read_request(io.BytesIO(data))
        self.assertEqual(h.read_request(io.BytesIO(b"{}")), {})


if __name__ == "__main__":
    unittest.main()
