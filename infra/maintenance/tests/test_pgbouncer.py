import json
import os
from pathlib import Path
import stat
import sys
import unittest
from unittest.mock import patch

from maintenance import host as h, plan as p, cli, report
import test_host
from test_controller import ready


class PoolTests(unittest.TestCase):
    setUp = test_host.HostTests.setUp
    read_file = test_host.HostTests.read_file
    readiness = test_host.HostTests.readiness
    request = test_host.HostTests.request
    mutations = test_host.HostTests.mutations

    def configure_pool(self):
        self.config.update(host_role="database", database_enabled=True,
                           backup_readiness=self.readiness(),
                           pgbouncer_health={"database": "fixture_db", "user": "fixture_user"})
        credential = Path("/etc/sifto-maintenance/pgbouncer-health.pgpass")
        credential.write_text("127.0.0.1:6432:fixture_db:fixture_user:fixture-password\n")
        credential.chmod(0o600)
        self.addCleanup(credential.unlink, missing_ok=True)
        previous = self.runner
        self.pool_output = "1|fixture_db\n"
        self.pool_error = False
        self.pool_env = None

        def run(argv, **kwargs):
            if argv[0] == "/usr/bin/psql":
                previous.calls.append(list(argv))
                previous.timeouts.append((list(argv), kwargs.get("timeout")))
                self.pool_env = kwargs.get("env")
                if self.pool_error:
                    raise p.MaintenanceError("fixture-password fixture_db 10.0.0.1")
                return self.pool_output
            return previous(argv, **kwargs)

        run.compare_versions = previous.compare_versions
        self.agent.runner = run
        return credential

    def test_unconfigured_pool_scan_retains_inventory_but_maintenance_is_denied(self):
        self.config.update(host_role="database", database_enabled=True, backup_readiness=self.readiness())
        inv = self.agent.run("scan", {})
        self.assertEqual(inv["runtime"].get("database_pool_health"), {"status": "unknown", "reason": "unconfigured"})
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", self.request())
        self.assertEqual(self.mutations(), [])

    def test_fixed_pool_read_checks_route_without_exporting_credentials(self):
        self.configure_pool()
        result = self.agent.run("apply", self.request(dry_run=True))
        self.assertEqual(result["status"], "preflight_passed")
        probes = [argv for argv in self.runner.calls if argv[0] == "/usr/bin/psql"]
        self.assertTrue(probes, "local postgres socket probe cannot validate PgBouncer")
        self.assertTrue(all("--host=127.0.0.1" in x and "--port=6432" in x and "--no-password" in x for x in probes))
        self.assertIn("current_database()", probes[0][-1])
        self.assertIn("BEGIN READ ONLY", probes[0][-1])
        self.assertEqual(self.pool_env["PGPASSFILE"], "/etc/sifto-maintenance/pgbouncer-health.pgpass")
        self.assertNotIn("PGPASSWORD", self.pool_env)
        self.assertTrue(all(timeout <= 30 for argv, timeout in self.runner.timeouts if argv[0] == "/usr/bin/psql"))
        inv = self.agent.run("scan", {})
        self.assertEqual(inv["runtime"]["database_pool_health"]["status"], "healthy")
        payload = report.ai_payload(p.build_plan(inv, self.config["approved_workflow_sha"], now=1000))
        for value in ("fixture-password", "fixture_db", "fixture_user", "127.0.0.1"):
            self.assertNotIn(value, json.dumps(inv))
            self.assertNotIn(value, json.dumps(payload))
            self.assertNotIn(value, json.dumps(probes) if value == "fixture-password" else "")

    def test_pool_auth_failure_and_route_mismatch_refuse_preflight(self):
        self.configure_pool()
        for auth_failed, output, reason in [(True, "", "probe_failed"), (False, "1|other_db\n", "route_mismatch")]:
            self.pool_error, self.pool_output = auth_failed, output
            inv = self.agent.run("scan", {})
            self.assertEqual(inv["runtime"].get("database_pool_health"), {"status": "failed", "reason": reason})
            with self.assertRaises(p.MaintenanceError) as error:
                self.agent.run("apply", self.request())
            self.assertNotIn("fixture-password", str(error.exception))
            self.assertEqual(self.mutations(), [])

    def test_credential_missing_public_or_symlink_refuses_probe(self):
        credential = self.configure_pool()
        for mode in (0o644, 0o400):
            credential.chmod(mode)
            self.assertEqual(self.agent.run("scan", {})["runtime"].get("database_pool_health"),
                             {"status": "unknown", "reason": "credentials_unavailable"})
        credential.unlink()
        self.assertEqual(self.agent.run("scan", {})["runtime"]["database_pool_health"]["status"], "unknown")
        credential.symlink_to("/etc/passwd")
        self.assertEqual(self.agent.run("scan", {})["runtime"]["database_pool_health"]["status"], "unknown")
        self.assertFalse(any(argv[0] == "/usr/bin/psql" for argv in self.runner.calls))

    def pool_package(self):
        self.configure_pool()
        self.config["allowed_restart_services"].append("pgbouncer.service")
        self.runner.extra_packages = "pgbouncer\t1.21.0-1ubuntu1\tinstalled\n"
        self.runner.simulation = "Inst pgbouncer [1.21.0-1ubuntu1] (1.22.1-1ubuntu1 Ubuntu:24.04/noble [arm64])\n"
        self.running_version = "1.21.0"
        probe = patch.object(h, "probe_running_pgbouncer", side_effect=lambda _: self.running_version, create=True)
        probe.start()
        self.addCleanup(probe.stop)

    def test_pool_update_cannot_complete_or_verify_stale_running_binary(self):
        self.pool_package()
        request = self.request()
        self.assertEqual(request["envelope"]["plan"]["expected_runtime"].get("pgbouncer_version"), "1.22.1")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        record = json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())
        self.assertEqual(record["status"], "failed")
        self.assertIn(["systemctl", "restart", "pgbouncer.service"], self.mutations())

    def test_signed_pool_plan_cannot_omit_runtime_target(self):
        self.pool_package()
        request = self.request(lambda plan: plan["expected_runtime"].pop("pgbouncer_version"))
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(list((self.root / "state" / "plans").glob("*.json")), [])

    def test_pool_new_runtime_completes_and_later_stale_verify_is_denied(self):
        self.pool_package()
        request = self.request()
        self.runner.before_install = lambda: setattr(self, "running_version", "1.22.1")
        self.assertEqual(self.agent.run("apply", request)["status"], "completed")
        self.running_version = "1.21.0"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("verify", {"plan_id": request["envelope"]["plan"]["plan_id"]})

    def test_pool_failure_after_install_is_journaled_failed(self):
        self.configure_pool()
        request = self.request()
        self.runner.before_install = lambda: setattr(self, "pool_error", True)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        record = json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())
        self.assertEqual(record["status"], "failed")

    def test_invalid_root_pool_identity_never_selects_remote_dsn_or_executes_probe(self):
        self.configure_pool()
        for config in ({"database": "postgresql://remote/db", "user": "fixture_user"},
                       {"database": "host=remote", "user": "fixture_user"},
                       {"database": "fixture_db", "user": "fixture_user", "host": "remote"},
                       {"database": "fixture_db", "user": "x\npassword"}):
            self.config["pgbouncer_health"] = config
            inv = self.agent.run("scan", {})
            self.assertEqual(inv["runtime"].get("database_pool_health"), {"status": "unknown", "reason": "invalid_configuration"})
        self.assertFalse(any(argv[0] == "/usr/bin/psql" for argv in self.runner.calls))

    def test_unavailable_pool_runtime_retains_inventory_but_denies_plan_and_apply(self):
        self.pool_package()
        self.running_version = None
        inv = self.agent.run("scan", {})
        self.assertIsNone(inv["runtime"]["pgbouncer_version"])
        self.assertTrue(inv["packages"])
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, self.config["approved_workflow_sha"], now=1000)

    def test_reboot_verify_requires_installed_pool_version_and_healthy_route(self):
        self.pool_package()
        self.runner.extra_packages = "pgbouncer\t1.22.1-1ubuntu1\tinstalled\n"
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], allow_reboot=True, now=1000, action="reboot")
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123", "approved_by": "human",
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"], "execution_mode": "apply"}
        request = {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key)}
        self.assertEqual(self.agent.run("apply", request)["status"], "reboot_scheduled")
        self.agent.read_file = lambda path: "boot-b\n" if str(path).endswith("boot_id") else self.read_file(path)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("verify", {"plan_id": plan["plan_id"]})
        self.running_version = "1.22.1"
        self.pool_error = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("verify", {"plan_id": plan["plan_id"]})
        self.pool_error = False
        self.assertEqual(self.agent.run("verify", {"plan_id": plan["plan_id"]})["status"], "verified")


class PoolPlanTests(unittest.TestCase):
    def test_running_pool_version_is_signed_identity_and_validated(self):
        inv = ready()
        inv["runtime"]["pgbouncer_version"] = "1.21.0"
        original = p.state_sha256(inv)
        plan = p.build_plan(inv, "a" * 40, now=1000)
        self.assertEqual(plan["expected_runtime"].get("pgbouncer_version"), "1.21.0")
        inv["runtime"]["pgbouncer_version"] = "1.22.1"
        self.assertNotEqual(p.state_sha256(inv), original)
        for value in (None, "1.2;id", "1", True):
            plan["expected_runtime"]["pgbouncer_version"] = value
            with self.assertRaises(p.MaintenanceError):
                p.validate_plan(plan)

    def test_reboot_and_explicit_restart_target_installed_pool_package(self):
        inv = ready()
        inv["runtime"]["pgbouncer_version"] = "1.21.0"
        inv["packages"].append({"name": "pgbouncer:arm64", "version": "1:1.22.1-1ubuntu1"})
        for action, restarts in [("reboot", []), ("apt_upgrade", ["pgbouncer.service"])]:
            self.assertEqual(p.expected_runtime(inv, [], action, restarts).get("pgbouncer_version"), "1.22.1")

    def test_missing_pool_runtime_cannot_approve_plan(self):
        inv = ready()
        inv["packages"].append({"name": "pgbouncer:arm64", "version": "1.21.0-1ubuntu1"})
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inv, "a" * 40, now=1000)

    def test_unconfigured_pool_is_unknown_in_controller_despite_fresh_backup(self):
        inv = ready()
        inv["maintenance_policy"].update(host_role="database", database_enabled=True, backup_ready=True)
        inv["runtime"]["database_pool_health"] = {"status": "unknown", "reason": "unconfigured"}
        result = cli.assess(inv, "a" * 40, now=1000)
        self.assertEqual((result["status"], result["reason"]), ("unknown", "database_pool_unconfigured"))
        self.assertIsNone(result["plan"])


class RunningPoolProbeTests(unittest.TestCase):
    def test_real_runner_executes_open_fd_without_inheriting_password_environment(self):
        descriptor = os.open(sys.executable, os.O_RDONLY | os.O_CLOEXEC)
        try:
            code = "import os; assert not any(k.startswith('PG') for k in os.environ); print('fd-execution-ok')"
            with patch.dict(os.environ, {"PGPASSWORD": "fixture-password", "PGHOST": "remote.invalid"}):
                output = h.SubprocessRunner()(["/proc/self/fd/" + str(descriptor), "-c", code],
                                              timeout=10, pass_fds=(descriptor,))
            self.assertEqual(output.strip(), "fd-execution-ok")
        finally:
            os.close(descriptor)

    def probe(self, *, pid="123", executable="/usr/sbin/pgbouncer", changed=False, uid=0):
        self.assertTrue(hasattr(h, "probe_running_pgbouncer"), "running process version probe is missing")
        info = os.stat_result((stat.S_IFREG | 0o755, 456, 1, 1, uid, 0, 0, 0, 0, 0))
        calls = []
        def runner(argv, **kwargs):
            calls.append((argv, kwargs))
            return pid if argv[0] == "systemctl" else "PgBouncer 1.25.1\n"
        prefix = "123 (pgbouncer) S " + "0 " * 18
        identities = [prefix + "100", prefix + ("101" if changed else "100")]
        with patch.object(h.os, "readlink", return_value=executable), patch.object(h.os, "open", return_value=77), \
             patch.object(h.os, "close"), patch.object(h.os, "fstat", return_value=info), \
             patch.object(h.os, "stat", return_value=info), \
             patch.object(h.Path, "read_text", side_effect=identities):
            result = h.probe_running_pgbouncer(runner)
        return result, calls

    def test_running_executable_fd_is_used_including_deleted_binary(self):
        for path in ("/usr/sbin/pgbouncer", "/usr/sbin/pgbouncer (deleted)"):
            result, calls = self.probe(executable=path)
            self.assertEqual(result, "1.25.1")
            self.assertEqual(calls[1][0], ["/proc/self/fd/77", "--version"])
            self.assertEqual(calls[1][1]["pass_fds"], (77,))
            self.assertLessEqual(calls[1][1]["timeout"], 30)

    def test_invalid_pid_untrusted_binary_or_process_change_is_denied(self):
        for args in ({"pid": "0"}, {"pid": "1;id"}, {"pid": "999999999999"},
                     {"executable": "/tmp/pgbouncer"}, {"uid": 1000}, {"changed": True}):
            with self.subTest(args=args), self.assertRaises(p.MaintenanceError):
                self.probe(**args)

