"""Synthetic control-flow tests. Only Docker/external storage are substituted."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import time
import tarfile
import sys
import stat
import signal
import unittest
from unittest.mock import patch


class ReadinessTests(unittest.TestCase):
    def setUp(self):
        script = Path(__file__).with_name("backup_readiness.py")
        self.assertTrue(script.exists(), "exact-snapshot offhost readiness automation is missing")
        spec = importlib.util.spec_from_file_location("proof", script)
        self.m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.m)
        self.temp = tempfile.TemporaryDirectory(dir="/var/lib/sifto-maintenance")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.state = self.root / "state"
        self.state.mkdir(mode=0o700)
        self.etc = self.root / "etc"
        self.etc.mkdir(mode=0o700)
        self.sources = self.root / "sources"
        self.sources.mkdir(mode=0o700)
        self.config = self.etc / "config.json"
        self.original = {"host_id": "database-fixture", "host_role": "database",
                         "database_enabled": False, "allowed_approvers": [],
                         "backup_destination": {"bucket": "sifto-backups-prod", "prefix": "database"},
                         "trusted_public_key": "preserve", "approved_workflow_sha": "preserve",
                         "other_field": {"nested": [1, 2]}, "backup_readiness": {"old": True}}
        self.write(self.config, json.dumps(self.original))
        self.env = self.etc / "backup.env"
        self.env_text = ("RESTIC_REPOSITORY=s3:https://" + "a" * 32 + ".r2.cloudflarestorage.com/sifto-backups-prod/database\n"
                         "RESTIC_PASSWORD=synthetic-independent-password\nAWS_ACCESS_KEY_ID=synthetic-key\n"
                         "AWS_SECRET_ACCESS_KEY=synthetic-secret\nAWS_DEFAULT_REGION=auto\n"
                         "BACKUP_DESTINATION_APPROVED=database\nBACKUP_HOST_ID=database-fixture\n")
        self.write(self.env, self.env_text)
        self.dump = self.sources / "sifto-20261011-030000.dump.gz"
        self.write(self.dump, "synthetic frozen dump bytes")
        for key, value in {"STATE": self.state, "CONFIG": self.config, "ENV_FILE": self.env,
                           "DB_SOURCES": self.sources, "SOURCE_UID": 0}.items():
            setattr(self.m, key, value)
        self.configure_compose(Path(__file__).with_name("compose.yml"))
        self.commands = []
        self.snapshot = "b" * 64
        self.corrupt = False
        self.restore_fail = False
        self.invalid_snapshot = False
        self.restored = []

    def write(self, path, value):
        path.write_text(value)
        path.chmod(0o600)

    def configure_compose(self, checkout_source):
        # Linux CI bind mounts retain checkout UID1001. Install only the
        # synthetic Compose fixture under root-owned private state, as an
        # operator would install the real protected file on a native host.
        target = self.state / "compose.yml"
        target.write_bytes(checkout_source.read_bytes())
        target.chmod(0o600)
        self.m.COMPOSE = target

    def foreign_checkout_compose(self):
        checkout = self.root / "uid1001-checkout"
        checkout.mkdir(mode=0o755)
        source = checkout / "compose.yml"
        source.write_bytes(Path(__file__).with_name("compose.yml").read_bytes())
        source.chmod(0o644)
        os.chown(source, 1001, 1001)
        os.chown(checkout, 1001, 1001)
        return source

    def test_protected_fixture_allows_checkout_owned_by_uid1001(self):
        self.configure_compose(self.foreign_checkout_compose())
        try:
            self.run_backup()
        except self.m.BackupError as exc:
            self.fail("fixture must use a root-protected Compose copy: " + str(exc))
        self.assertEqual(self.m.COMPOSE.stat().st_uid, 0)
        self.assertEqual(self.m.COMPOSE.stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.m.COMPOSE.parent, self.state)

    def test_unprotected_checkout_compose_still_rejected_before_egress(self):
        self.m.COMPOSE = self.foreign_checkout_compose()
        with self.assertRaisesRegex(self.m.BackupError, "unprotected path ancestor"):
            self.run_backup()
        self.assertEqual(self.commands, [])
        self.assert_old()

    def transport(self, command, **kwargs):
        self.commands.append(command)
        if "backup" in command and "--json" in command:
            return json.dumps({"message_type": "summary", "snapshot_id": "latest" if self.invalid_snapshot else self.snapshot}) + "\n"
        if "restore" in command:
            target = Path(kwargs["env"]["MAINTENANCE_RESTORE_TARGET"]) / "source"
            target.mkdir()
            for p in Path(kwargs["env"]["MAINTENANCE_BACKUP_SOURCE"]).iterdir():
                (target / p.name).write_bytes(b"corrupted" if self.corrupt else p.read_bytes())
        return ""

    def restore(self, role, dump, work):
        self.restored.append(Path(dump).read_bytes())
        if self.restore_fail:
            raise self.m.BackupError("full restore failed")
        return "sha256:" + "c" * 64

    def run_backup(self):
        with patch.object(self.m, "run", self.transport), patch.object(self.m, "restore_database", self.restore):
            self.m.backup("database")

    def assert_old(self):
        self.assertEqual(json.loads(self.config.read_text()), self.original)
        self.assertFalse(list(self.state.glob("evidence/offhost-*.json")))

    def subprocess_family(self, action, *, detached_output=False):
        pidfile = self.state / "descendant.pid"
        child = ("import os, pathlib, signal, sys, time; signal.signal(signal.SIGTERM, signal.SIG_IGN); "
                 + ("os.close(1); os.close(2); " if detached_output else "")
                 + "pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); time.sleep(30)")
        parent = ("import pathlib, subprocess, sys, time, os; "
                  "subprocess.Popen([sys.executable, '-c', " + repr(child) + ", sys.argv[1]]); "
                  "pidfile = pathlib.Path(sys.argv[1]); "
                  "\nwhile not pidfile.exists(): time.sleep(0.01)\n" + action)
        def cleanup():
            if pidfile.exists():
                pid = int(pidfile.read_text())
                cmdline = Path("/proc") / str(pid) / "cmdline"
                if cmdline.exists() and str(pidfile).encode() in cmdline.read_bytes():
                    try:
                        os.kill(pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
        self.addCleanup(cleanup)
        return [sys.executable, "-c", parent, str(pidfile)], pidfile

    def assert_descendant_stopped(self, pidfile):
        self.assertTrue(pidfile.exists(), "real descendant must start before the supervision case")
        pid = int(pidfile.read_text())
        status = Path("/proc") / str(pid) / "stat"
        deadline = time.monotonic() + 1
        while status.exists() and status.read_text().split()[2] != "Z" and time.monotonic() < deadline:
            time.sleep(0.01)
        self.assertTrue(not status.exists() or status.read_text().split()[2] == "Z", "subprocess descendant remains running")

    def test_run_allows_producer_to_create_dump_larger_than_output_limit(self):
        target = self.state / "synthetic-large.dump"
        try:
            result = self.m.run([sys.executable, "-c", "import pathlib, sys; pathlib.Path(sys.argv[1]).write_bytes(b'x' * (3*1024*1024)); print('created')", str(target)])
        except self.m.BackupError:
            self.fail("producer ordinary file must exceed the stdout-only limit; file stopped at " + str(target.stat().st_size))
        self.assertEqual(result, "created\n")
        self.assertEqual(target.stat().st_size, 3*1024*1024)

    def test_run_bounds_combined_stdout_stderr_without_truncating_valid_output(self):
        result = self.m.run([sys.executable, "-c", "import os; os.write(1,b'o'*2048); os.write(2,b'e'*2048)"], limit=4096)
        self.assertEqual(result, "o"*2048 + "e"*2048)

    def test_run_output_overflow_promptly_kills_descendants_without_exposing_output(self):
        command, pidfile = self.subprocess_family("os.write(2, b'synthetic-private-output');\nwhile True: os.write(1, b'x'*4096)")
        before = time.monotonic()
        with self.assertRaises(self.m.BackupError) as error:
            self.m.run(command, timeout=3, limit=4096)
        self.assertLess(time.monotonic()-before, 2)
        self.assertNotIn("synthetic-private-output", str(error.exception))
        self.assert_descendant_stopped(pidfile)

    def test_run_timeout_kills_descendants_without_exposing_stderr(self):
        command, pidfile = self.subprocess_family("os.write(2, b'synthetic-private-output'); time.sleep(30)")
        before = time.monotonic()
        with self.assertRaises(self.m.BackupError) as error:
            self.m.run(command, timeout=0.6)
        self.assertLess(time.monotonic()-before, 3)
        self.assertNotIn("synthetic-private-output", str(error.exception))
        self.assert_descendant_stopped(pidfile)

    def test_run_times_out_when_exited_parent_leaves_descendant_holding_output_pipe(self):
        command, pidfile = self.subprocess_family("sys.exit(0)")
        with self.assertRaises(self.m.BackupError):
            self.m.run(command, timeout=0.6)
        self.assert_descendant_stopped(pidfile)

    def test_run_success_also_cleans_up_descendant_with_closed_output(self):
        command, pidfile = self.subprocess_family("print('complete')", detached_output=True)
        self.assertEqual(self.m.run(command, timeout=3), "complete\n")
        self.assert_descendant_stopped(pidfile)

    def test_run_nonzero_exit_and_invalid_utf8_fail_without_echoing_child_output(self):
        for code in ("import os,sys; os.write(2,b'synthetic-private-output'); sys.exit(7)",
                     "import os; os.write(1,b'\\xff')"):
            with self.assertRaises(self.m.BackupError) as error:
                self.m.run([sys.executable, "-c", code])
            self.assertNotIn("synthetic-private-output", str(error.exception))

    def test_retrieved_full_restore_publishes_preserving_all_config_fields(self):
        self.run_backup()
        updated = json.loads(self.config.read_text())
        self.assertEqual({k: v for k, v in updated.items() if k != "backup_readiness"},
                         {k: v for k, v in self.original.items() if k != "backup_readiness"})
        proof = updated["backup_readiness"]
        self.assertEqual(proof["backup_id"], self.snapshot)
        self.assertEqual(proof["offhost_object"], "s3://sifto-backups-prod/database/snapshots/" + self.snapshot)
        self.assertEqual(self.restored, [self.dump.read_bytes()])
        self.assertEqual(self.config.stat().st_mode & 0o777, 0o600)
        sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
        from maintenance.host import HostAgent
        agent = object.__new__(HostAgent)
        agent.config = updated
        agent.clock = time.time
        self.assertTrue(agent._backup_ready(), "published proof must satisfy the native DB readiness gate")
        restore = next(c for c in self.commands if "restore" in c)
        self.assertIn(self.snapshot, restore)
        self.assertNotIn("latest", restore)
        forget = next(c for c in self.commands if "forget" in c)
        self.assertIn("--host", forget)
        self.assertIn("--tag", forget)
        self.assertIn("14d", forget)

    def test_wrong_retrieved_hash_preserves_old_proof(self):
        self.corrupt = True
        with self.assertRaises(self.m.BackupError):
            self.run_backup()
        self.assert_old()
        self.assertEqual(self.restored, [])

    def test_failed_full_restore_preserves_old_proof(self):
        self.restore_fail = True
        with self.assertRaises(self.m.BackupError):
            self.run_backup()
        self.assert_old()

    def test_invalid_snapshot_never_retrieves_or_freshens(self):
        self.invalid_snapshot = True
        with self.assertRaises(self.m.BackupError):
            self.run_backup()
        self.assert_old()
        self.assertFalse(any("restore" in c for c in self.commands))

    def test_expired_source_never_sends_bytes(self):
        os.utime(self.dump, (time.time() - 90000,) * 2)
        with self.assertRaises(self.m.BackupError):
            self.run_backup()
        self.assertEqual(self.commands, [])
        self.assert_old()

    def test_unapproved_destination_and_env_code_fail_before_egress(self):
        for text in [self.env_text.replace("APPROVED=database", "APPROVED=pending"),
                     self.env_text + "MALICIOUS=$(touch /tmp/untrusted-env-executed)\n",
                     self.env_text.replace("sifto-backups-prod", "REPLACE_BUCKET")]:
            self.write(self.env, text)
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
            self.assertEqual(self.commands, [])
            self.assert_old()

    def test_destination_without_root_policy_fails_before_backup_or_init_egress(self):
        self.original.pop("backup_destination")
        self.write(self.config, json.dumps(self.original))
        with self.assertRaises(self.m.BackupError):
            self.run_backup()
        with patch.object(self.m, "run", self.transport):
            with self.assertRaises(self.m.BackupError):
                self.m.initialize("database", create=True)
        self.assertEqual(self.commands, [])
        self.assert_old()

    def test_shared_bucket_or_wrong_prefix_is_rejected_even_if_root_approved(self):
        for bucket, prefix in (("sifto-podcast-prod", "database"), ("other-private-bucket", "database"),
                               ("sifto-backups-prod", "application"), ("sifto-backups-prod", "database/other")):
            config = {**self.original, "backup_destination": {"bucket": bucket, "prefix": prefix}}
            self.write(self.config, json.dumps(config))
            self.write(self.env, self.env_text.replace("sifto-backups-prod/database", bucket + "/" + prefix))
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
            self.assertEqual(self.commands, [])
            self.assertEqual(json.loads(self.config.read_text()), config)

    def test_destination_must_match_exact_approved_root_schema_and_repository(self):
        self.write(self.env, self.env_text.replace("sifto-backups-prod", "other-private-bucket"))
        for destination in (None, "sifto-backups-prod", {"bucket": "sifto-backups-prod"},
                            {"bucket": "sifto-backups-prod", "prefix": "database", "host": "unexpected"},
                            {"bucket": "sifto-backups-prod", "prefix": "database"}):
            config = {**self.original, "backup_destination": destination}
            self.write(self.config, json.dumps(config))
            # The final policy is valid but the env still selects another bucket.
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
            self.assertEqual(self.commands, [])
            self.assertEqual(json.loads(self.config.read_text()), config)

        self.write(self.config, json.dumps(self.original))
        for repository in ("sifto-podcast-prod/database", "sifto-backups-prod/application",
                           "sifto-backups-prod/database/other"):
            self.write(self.env, self.env_text.replace("sifto-backups-prod/database", repository))
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
            self.assertEqual(self.commands, [])
            self.assert_old()

    def test_destination_approval_revoked_during_restore_never_publishes_readiness(self):
        configured = {**self.original, "backup_destination": {"bucket": "sifto-backups-prod", "prefix": "database"}}
        self.write(self.config, json.dumps(configured))
        self.write(self.env, self.env_text)
        def revoke(role, dump, work):
            current = json.loads(self.config.read_text())
            current.pop("backup_destination")
            self.write(self.config, json.dumps(current))
            return self.restore(role, dump, work)
        with patch.object(self.m, "run", self.transport), patch.object(self.m, "restore_database", revoke):
            with self.assertRaises(self.m.BackupError):
                self.m.backup("database")
        current = json.loads(self.config.read_text())
        self.assertNotIn("backup_destination", current)
        self.assertEqual(current["backup_readiness"], self.original["backup_readiness"])
        self.assertFalse(list(self.state.glob("evidence/offhost-*.json")))

    def test_maintenance_lock_prevents_publication(self):
        with self.m.lock(self.state / "maintenance.lock"):
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
        self.assert_old()

    def test_source_lock_prevents_egress(self):
        with self.m.lock(self.sources / ".sifto-backup.lock"):
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
        self.assertEqual(self.commands, [])

    def test_symlink_env_parent_is_rejected(self):
        linked = self.root / "linked"
        linked.symlink_to(self.etc, target_is_directory=True)
        self.m.ENV_FILE = linked / "backup.env"
        with self.assertRaises(self.m.BackupError):
            self.run_backup()
        self.assertEqual(self.commands, [])

    def test_init_checks_existing_repository_without_unlock_or_reinitialization(self):
        with patch.object(self.m, "run", self.transport):
            self.m.initialize("database")
        self.assertEqual(len(self.commands), 1)
        self.assertIn("snapshots", self.commands[0])
        self.assertFalse(any("init" in c or "unlock" in c for c in self.commands))

    def test_explicit_create_is_idempotent_for_an_existing_repository(self):
        with patch.object(self.m, "run", self.transport):
            self.m.initialize("database", create=True)
        self.assertEqual(len(self.commands), 1)
        self.assertIn("snapshots", self.commands[0])

    def test_retention_failure_does_not_publish_new_proof(self):
        def failed_retention(command, **kwargs):
            if "forget" in command:
                raise self.m.BackupError("retention failed")
            return self.transport(command, **kwargs)
        with patch.object(self.m, "run", failed_retention), patch.object(self.m, "restore_database", self.restore):
            with self.assertRaises(self.m.BackupError):
                self.m.backup("database")
        self.assert_old()

    def test_config_write_failure_keeps_original_file_complete(self):
        original = self.config.read_bytes()
        with patch.object(self.m.os, "fsync", side_effect=OSError("disk failure")):
            with self.assertRaises(OSError):
                self.m.atomic_json(self.config, {"truncated": True})
        self.assertEqual(self.config.read_bytes(), original)
        self.assertFalse(list(self.etc.glob(".proof.*")))

    def test_post_replace_config_directory_fsync_failure_does_not_advance_readiness(self):
        fsync = self.m.os.fsync
        failed = False
        def fail_after_config_replacement(fd):
            nonlocal failed
            info = os.fstat(fd)
            if (not failed and stat.S_ISDIR(info.st_mode)
                    and info.st_ino == self.etc.stat().st_ino
                    and json.loads(self.config.read_text()) != self.original):
                failed = True
                raise OSError("post-replace configuration directory fsync failure")
            return fsync(fd)
        with patch.object(self.m.os, "fsync", fail_after_config_replacement):
            with self.assertRaises((OSError, self.m.BackupError)):
                self.run_backup()
        self.assertTrue(failed, "exercise failure after configuration replacement")
        self.assertEqual(json.loads(self.config.read_text()), self.original)
        # Standalone evidence records a real verified restore; it may remain.
        self.assertEqual(len(list(self.state.glob("evidence/offhost-*.json"))), 1)
        self.assertEqual(self.config.stat().st_mode & 0o777, 0o600)
        self.assertFalse(list(self.etc.glob(".rollback.*")))

    def test_default_maintenance_target_runs_backup_unit_tests(self):
        import re
        targets = {name: (deps.split(), recipe) for name, deps, recipe in re.findall(
            r"^([a-zA-Z0-9_-]+):([^\n]*)\n((?:\t[^\n]*\n)*)", Path("/repository/Makefile").read_text(), re.M)}
        def reachable_commands(target):
            deps, recipe = targets[target]
            return recipe + "".join(reachable_commands(dep) for dep in deps if dep in targets)
        self.assertIn("backup/test_backup_readiness.py", reachable_commands("maintenance-test"))

    def test_failed_rollback_rename_disables_readiness_and_preserves_private_recovery_file(self):
        fsync, replace = self.m.os.fsync, self.m.os.replace
        failed = False
        def fail_config_directory_fsync(fd):
            nonlocal failed
            info = os.fstat(fd)
            if (not failed and stat.S_ISDIR(info.st_mode) and info.st_ino == self.etc.stat().st_ino
                    and json.loads(self.config.read_text()) != self.original):
                failed = True
                raise OSError("configuration commit fsync failure")
            return fsync(fd)
        def fail_rollback(source, target):
            if Path(source).name.startswith(".rollback."):
                raise OSError("rollback rename unavailable")
            return replace(source, target)
        with patch.object(self.m.os, "fsync", fail_config_directory_fsync), patch.object(self.m.os, "replace", fail_rollback):
            with self.assertRaises((OSError, self.m.BackupError)):
                self.run_backup()
        self.assertEqual(self.config.read_bytes(), b"", "unavailable rollback must deny config loading/readiness")
        recovery = list(self.etc.glob(".rollback.*"))
        self.assertEqual(len(recovery), 1)
        self.assertEqual(json.loads(recovery[0].read_text()), self.original)
        self.assertEqual(recovery[0].stat().st_mode & 0o777, 0o600)

    def test_persistent_directory_fsync_failure_keeps_old_config_after_rollback(self):
        fsync = self.m.os.fsync
        failed = False
        def persistent_config_directory_failure(fd):
            nonlocal failed
            info = os.fstat(fd)
            if stat.S_ISDIR(info.st_mode) and info.st_ino == self.etc.stat().st_ino:
                if failed or json.loads(self.config.read_text()) != self.original:
                    failed = True
                    raise OSError("persistent directory fsync failure")
            return fsync(fd)
        with patch.object(self.m.os, "fsync", persistent_config_directory_failure):
            with self.assertRaisesRegex(self.m.BackupError, "previous configuration restored"):
                self.run_backup()
        self.assertTrue(failed)
        self.assertEqual(json.loads(self.config.read_text()), self.original)
        self.assertFalse(list(self.etc.glob(".rollback.*")))

    def test_missing_postgres_owned_lock_is_not_created_as_root(self):
        os.chown(self.sources, 999, 999)
        path = self.sources / "missing.lock"
        with self.assertRaises(self.m.BackupError):
            with self.m.lock(path, uid=999):
                pass
        self.assertFalse(path.exists())

    def test_concurrent_configuration_changes_are_preserved_at_publication(self):
        def restore_and_change(role, dump, work):
            self.original["new_operator_field"] = "preserve latest under lock"
            self.write(self.config, json.dumps(self.original))
            return self.restore(role, dump, work)
        with patch.object(self.m, "run", self.transport), patch.object(self.m, "restore_database", restore_and_change):
            self.m.backup("database")
        self.assertEqual(json.loads(self.config.read_text())["new_operator_field"], self.original["new_operator_field"])

    def test_reading_source_may_update_access_time_without_invalidating_backup(self):
        os.utime(self.dump, (time.time() - 3600, time.time()))
        self.run_backup()
        self.assertEqual(len(self.restored), 1)

    def test_unsafe_coolify_tar_never_executes_or_extracts_content(self):
        for name, kind in [("../escape", tarfile.REGTYPE), ("source/link", tarfile.SYMTYPE), ("/absolute", tarfile.REGTYPE)]:
            archive = self.state / "config.tar.gz"
            with tarfile.open(archive, "w:gz") as tar:
                member = tarfile.TarInfo(name)
                member.type = kind
                member.linkname = "/etc/passwd"
                tar.addfile(member)
            with self.assertRaises(self.m.BackupError):
                self.m.verify_config_archive(archive)
        self.assertFalse((self.root / "escape").exists())

    def test_coolify_archive_must_contain_all_recovery_roots(self):
        archive = self.state / "empty.tar.gz"
        with tarfile.open(archive, "w:gz"):
            pass
        with self.assertRaises(self.m.BackupError):
            self.m.verify_config_archive(archive)

    def test_source_content_change_during_staging_fails_closed(self):
        copy = self.m.shutil.copy2
        def changing_source(source, target):
            copy(source, target)
            Path(source).write_text("changed source bytes")
        with patch.object(self.m.shutil, "copy2", changing_source):
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
        self.assertEqual(self.commands, [])
        self.assert_old()

    def test_coolify_source_parent_symlink_is_rejected_before_generation(self):
        self.original["host_role"] = "application"
        self.original["backup_destination"]["prefix"] = "application"
        self.write(self.config, json.dumps(self.original))
        self.write(self.env, self.env_text.replace("APPROVED=database", "APPROVED=application").replace("/database", "/application"))
        linked = self.root / "coolify-source"
        linked.symlink_to(self.sources, target_is_directory=True)
        self.m.COOLIFY_SOURCE = linked
        with patch.object(self.m, "run", self.transport), patch.object(self.m, "stage", side_effect=self.m.BackupError("reached generation")):
            with self.assertRaises(self.m.BackupError):
                self.m.backup("application")
        self.assertEqual(self.commands, [])
        self.assert_old()

    def test_controlled_coolify_uid9999_source_layout_allows_generation(self):
        self.original["host_role"] = "application"
        self.original["backup_destination"]["prefix"] = "application"
        self.write(self.config, json.dumps(self.original))
        self.write(self.env, self.env_text.replace("APPROVED=database", "APPROVED=application").replace("/database", "/application"))
        source = self.root / "coolify-source"
        source.mkdir(mode=0o700)
        for name in ("source", "ssh", "ssl", "proxy"):
            child = source / name
            child.mkdir(mode=0o700)
            os.chown(child, 9999, 0)
        os.chown(source, 9999, 0)
        self.m.COOLIFY_SOURCE = source
        with patch.object(self.m, "run", self.transport), patch.object(self.m, "stage", side_effect=self.m.BackupError("stop after generation")):
            with self.assertRaises(self.m.BackupError):
                self.m.backup("application")
        self.assertEqual(self.commands, [["/bin/bash", "/opt/sifto-maintenance/backup/coolify-backup.sh"]])
        self.assert_old()

    def test_restore_container_collision_never_removes_existing_container(self):
        def collision(command, **kwargs):
            self.commands.append(command)
            if "inspect" in command:
                return "sha256:" + "c" * 64
            raise self.m.BackupError("container already exists")
        with patch.object(self.m, "run", collision):
            with self.assertRaises(self.m.BackupError):
                self.m.restore_database("database", self.dump, self.state)
        self.assertFalse(any("rm" in c for c in self.commands))

    def test_full_restore_uses_fixed_isolated_container_and_cleans_only_its_container(self):
        def docker(command, **kwargs):
            self.commands.append(command)
            if "inspect" in command:
                return "sha256:" + "c" * 64
            if "psql" in command:
                return "1\n"
            return ""
        with patch.object(self.m, "run", docker), patch.object(self.m, "restore_archive") as restore:
            self.m.restore_database("database", self.dump, self.state)
        create = next(c for c in self.commands if "run" in c)
        self.assertEqual(create[create.index("--network") + 1], "none")
        self.assertEqual(create[create.index("--cpus") + 1], "1")
        self.assertEqual(create[create.index("--memory") + 1], "768m")
        self.assertTrue(any("postgres:16.13-bookworm" in c for c in self.commands))
        self.assertIn("--exit-on-error", restore.call_args.args[2])
        self.assertTrue(any(c[-3:] == ["rm", "-f", "sifto-offhost-restore-database"] for c in self.commands))


if __name__ == "__main__":
    unittest.main()
