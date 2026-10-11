import copy
import io
import json
import tempfile
import unittest
import fcntl
import multiprocessing
import shutil
import subprocess
import sys
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
        self.timeouts = []
        self.simulation = SIMULATION
        self.upgraded = False
        self.fail_install = False
        self.service_active = True
        self.kernel = "6.8.0-59-generic"
        self.container_health = "healthy"
        self.before_install = None
        self.extra_packages = ""
        self.before_simulation = None
        self.installed_overrides = {}
        self.docker_version = "28.0.0"
        self.pg_version_num = "160004"
        self.image_id = "sha256:" + "d" * 64

    def __call__(self, argv, *, timeout=60, check=True):
        self.calls.append(list(argv))
        self.timeouts.append((list(argv), timeout))
        if argv[0] == "dpkg-query":
            packages = {"openssl": "3.0.13-0ubuntu3", "libssl3t64:arm64": "3.0.13-0ubuntu3"}
            for line in self.extra_packages.splitlines():
                name, version, _ = line.split("\t")
                packages[name] = version
            packages.update(self.installed_overrides)
            return "".join(f"{name}\t{version}\tinstalled\n" for name, version in packages.items())
        if argv[:2] == ["dpkg", "--print-architecture"]:
            return "arm64\n"
        if argv[:2] == ["dpkg", "--compare-versions"]:
            return "false"  # Real runner exposes return code through check=False? use helper below.
        if argv[:2] == ["uname", "-r"]:
            return self.kernel + "\n"
        if argv[:2] == ["docker", "version"]:
            return json.dumps({"Version": self.docker_version}) + "\n"
        if argv[:2] == ["docker", "ps"]:
            return "c" * 64 + "\n"
        if argv[:2] == ["docker", "inspect"]:
            return json.dumps({"id": "c" * 64, "image_id": self.image_id,
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
            self.installed_overrides.update(arg.split("=", 1) for arg in argv if "=" in arg and not arg.startswith(("DPkg::", "Acquire::", "APT::")))
            return "done"
        if argv[0] == "apt-get" and "update" in argv:
            return "done"
        if "psql" in " ".join(argv):
            if "server_version_num" in " ".join(argv):
                return self.pg_version_num + "\n"
            return "1\n"
        if argv[:2] == ["systemctl", "reboot"]:
            return ""
        if argv[:2] == ["systemctl", "restart"]:
            return ""
        raise AssertionError(f"unexpected command: {argv}")

    def compare_versions(self, left, operator, right):
        if operator == "gt":
            return left > right
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
                       "host_role": "application",
                       "health_wait_seconds": 0,
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

    def test_scan_exposes_typed_maintenance_policy_without_backup_object_details(self):
        self.config.update(host_role="database", database_enabled=False, backup_readiness=self.readiness())
        inventory = self.agent.run("scan", {})
        self.assertIn("maintenance_policy", inventory)
        self.assertEqual(inventory["maintenance_policy"], {"host_role": "database", "database_enabled": False,
                                                         "backup_ready": True})
        self.config["backup_readiness"]["offhost_verified_at"] = -90000
        self.assertFalse(self.agent.run("scan", {})["maintenance_policy"]["backup_ready"])

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

    def test_unprovisioned_dry_run_does_not_create_any_state(self):
        inv = self.agent.inventory()
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], now=1000)
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123", "approved_by": "human",
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"]}
        request = {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key), "dry_run": True}
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertFalse((self.root / "state").exists())
        self.assertEqual(self.mutations(), [])

    def test_needrestart_does_not_automatically_restart_unlisted_services(self):
        self.assertEqual(h.SubprocessRunner.ENV["NEEDRESTART_MODE"], "l")

    def test_package_manager_mutation_has_no_process_killing_timeout(self):
        self.agent.run("apply", self.request())
        mutation_timeouts = [timeout for argv, timeout in self.runner.timeouts
                             if argv[0] == "apt-get" and "install" in argv and "-s" not in argv]
        self.assertEqual(mutation_timeouts, [None])

    def test_post_upgrade_health_waits_for_starting_container(self):
        request = self.request()
        self.config["health_wait_seconds"] = 5
        self.runner.before_install = lambda: setattr(self.runner, "container_health", "starting")
        self.agent.sleeper = lambda _: setattr(self.runner, "container_health", "healthy")
        try:
            result = self.agent.run("apply", request)
        except p.MaintenanceError as exc:
            self.fail(f"transient starting container was not given a bounded health wait: {exc}")
        self.assertEqual(result["status"], "completed")
        self.assertEqual(len(self.mutations()), 1)

    def readiness(self):
        return {"backup_id": "backup-1", "verified_at": 900, "restore_tested_at": 800,
                "host_id": "sifto-app", "offhost_object": "s3://private-backups/db/backup-1.dump",
                "offhost_sha256": "e" * 64, "offhost_verified_at": 900}

    def test_database_role_rejects_library_upgrade_without_backup_even_when_enabled(self):
        self.config["host_role"] = "database"
        self.config["database_enabled"] = True
        request = self.request()
        request["dry_run"] = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_role_rejects_all_upgrade_when_maintenance_disabled(self):
        self.config["host_role"] = "database"
        self.config["backup_readiness"] = self.readiness()
        request = self.request()
        request["dry_run"] = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_role_requires_current_offhost_evidence(self):
        self.config["host_role"] = "database"
        self.config["database_enabled"] = True
        request = self.request()
        request["dry_run"] = True
        for readiness in [{"backup_id": "backup-1", "verified_at": 900, "restore_tested_at": 800, "host_id": "sifto-app"},
                          {**self.readiness(), "offhost_verified_at": -90000},
                          {**self.readiness(), "offhost_sha256": "not-a-hash"}]:
            self.config["backup_readiness"] = readiness
            with self.subTest(readiness=readiness), self.assertRaises(p.MaintenanceError):
                self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_role_valid_backup_allows_preflight_and_read_only_query(self):
        self.config["host_role"] = "database"
        self.config["database_enabled"] = True
        self.config["backup_readiness"] = self.readiness()
        request = self.request()
        request["dry_run"] = True
        self.assertEqual(self.agent.run("apply", request)["status"], "preflight_passed")
        self.assertTrue(any("BEGIN READ ONLY" in " ".join(x) for x in self.runner.calls))
        self.assertEqual(self.mutations(), [])

    def test_docker_upgrade_cannot_complete_with_old_running_engine(self):
        self.runner.extra_packages = "docker-ce\t5:28.0.0-1~ubuntu.24.04~noble\tinstalled\n"
        self.runner.simulation = "Inst docker-ce [5:28.0.0-1~ubuntu.24.04~noble] (5:28.1.1-1~ubuntu.24.04~noble Docker:24.04/noble [arm64])\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())["status"], "failed")

    def test_postgres_upgrade_cannot_complete_with_old_running_server(self):
        self.config.update(host_role="database", database_enabled=True, backup_readiness=self.readiness())
        self.runner.extra_packages = "postgresql-16\t16.4-1\tinstalled\n"
        self.runner.simulation = "Inst postgresql-16 [16.4-1] (16.5-1 Ubuntu:24.04/noble [arm64])\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())["status"], "failed")

    def test_reboot_verify_requires_latest_installed_kernel(self):
        self.runner.extra_packages = "linux-image-6.8.0-60-generic\t6.8.0-60.63\tinstalled\n"
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], allow_reboot=True, now=1000, action="reboot")
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123", "approved_by": "human",
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"]}
        request = {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key)}
        self.agent.run("apply", request)
        self.agent.read_file = lambda path: "boot-b\n" if str(path).endswith("boot_id") else self.read_file(path)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("verify", {"plan_id": plan["plan_id"]})
        self.runner.kernel = "6.8.0-60-generic"
        self.assertEqual(self.agent.run("verify", {"plan_id": plan["plan_id"]})["status"], "verified")

    def test_bootstrap_refuses_local_code_import_before_ownership_validation(self):
        script = self.root / "host_agent.py"
        shutil.copyfile(Path(__file__).parents[1] / "host_agent.py", script)
        evil = self.root / "maintenance"
        evil.mkdir()
        marker = self.root / "evil-imported"
        (evil / "__init__.py").write_text(f"from pathlib import Path\nPath({str(marker)!r}).touch()\n")
        result = subprocess.run([sys.executable, "-I", str(script), "scan-gateway"], capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(marker.exists(), "untrusted Python package executed before ownership check")

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

    def test_same_container_id_with_changed_image_identity_fails_health(self):
        request = self.request()
        self.runner.before_install = lambda: setattr(self.runner, "image_id", "sha256:" + "f" * 64)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(len(self.mutations()), 1)

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

    def test_real_posix_lock_is_detected_without_removing_lockfile(self):
        lockfile = self.root / "dpkg.lock"
        lockfile.touch()
        ready_parent, ready_child = multiprocessing.Pipe()

        def hold_lock():
            with lockfile.open("r+") as stream:
                fcntl.lockf(stream, fcntl.LOCK_EX)
                ready_child.send("locked")
                ready_child.recv()

        process = multiprocessing.Process(target=hold_lock)
        process.start()
        try:
            self.assertTrue(ready_parent.poll(5), "lock fixture did not start")
            self.assertEqual(ready_parent.recv(), "locked")
            with patch.object(h, "APT_LOCKS", (str(lockfile),)):
                self.assertEqual(h.probe_apt_locks(), [str(lockfile)])
            self.assertTrue(lockfile.is_file())
            ready_parent.send("release")
            process.join(timeout=5)
            self.assertEqual(process.exitcode, 0)
            with patch.object(h, "APT_LOCKS", (str(lockfile),)):
                self.assertEqual(h.probe_apt_locks(), [])
        finally:
            if process.is_alive():
                ready_parent.send("release")
                process.join(timeout=5)
            ready_parent.close()
            ready_child.close()

    def test_maintenance_lock_prevents_second_executor_and_can_be_released(self):
        other = h.HostAgent(self.config, self.runner, clock=lambda: 1000,
                            read_file=self.read_file, lock_probe=lambda: [])
        with self.agent.locked():
            with self.assertRaises(p.MaintenanceError):
                other.run("refresh", {})
        self.assertEqual(self.mutations(), [])
        self.assertEqual(other.run("refresh", {})["status"], "metadata_refreshed")

    def test_private_state_and_receipts_have_root_only_modes(self):
        request = self.request()
        self.agent.run("apply", request)
        self.assertEqual((self.root / "state").stat().st_mode & 0o777, 0o700)
        receipt = next((self.root / "state" / "plans").glob("*.json"))
        self.assertEqual(receipt.stat().st_mode & 0o777, 0o600)

    def test_refresh_partial_repository_failure_does_not_record_freshness(self):
        previous = self.runner

        def fail_update(argv, **kwargs):
            if argv[0] == "apt-get" and "update" in argv:
                raise p.MaintenanceError("repository unavailable")
            return previous(argv, **kwargs)

        self.agent.runner = fail_update
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("refresh", {})
        self.assertFalse((self.root / "state" / "refresh.json").exists())

    def test_stale_metadata_refuses_apply(self):
        request = self.request()
        (self.root / "state" / "refresh.json").write_text('{"refreshed_at": -90000}')
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_symlink_state_directory_is_rejected(self):
        target = self.root / "elsewhere"
        target.mkdir(mode=0o700)
        (self.root / "state").symlink_to(target, target_is_directory=True)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("refresh", {})
        self.assertEqual(self.mutations(), [])

    def test_secure_path_rejects_symlink_and_writable_ancestor(self):
        config = self.root / "config.json"
        config.write_text("{}")
        config.chmod(0o600)
        with self.assertRaises(p.MaintenanceError):
            h.secure_path(config)  # /tmp is writable by other users.
        link = self.root / "config-link.json"
        link.symlink_to(config)
        with self.assertRaises(p.MaintenanceError):
            h.secure_path(link)


if __name__ == "__main__":
    unittest.main()
