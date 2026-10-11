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
                         "trusted_public_key": "preserve", "approved_workflow_sha": "preserve",
                         "other_field": {"nested": [1, 2]}, "backup_readiness": {"old": True}}
        self.write(self.config, json.dumps(self.original))
        self.env = self.etc / "backup.env"
        self.env_text = ("RESTIC_REPOSITORY=s3:https://" + "a" * 32 + ".r2.cloudflarestorage.com/private-bucket/sifto/database\n"
                         "RESTIC_PASSWORD=synthetic-independent-password\nAWS_ACCESS_KEY_ID=synthetic-key\n"
                         "AWS_SECRET_ACCESS_KEY=synthetic-secret\nAWS_DEFAULT_REGION=auto\n"
                         "BACKUP_DESTINATION_APPROVED=database\nBACKUP_HOST_ID=database-fixture\n")
        self.write(self.env, self.env_text)
        self.dump = self.sources / "sifto-20261011-030000.dump.gz"
        self.write(self.dump, "synthetic frozen dump bytes")
        for key, value in {"STATE": self.state, "CONFIG": self.config, "ENV_FILE": self.env,
                           "DB_SOURCES": self.sources, "SOURCE_UID": 0,
                           "COMPOSE": Path(__file__).with_name("compose.yml")}.items():
            setattr(self.m, key, value)
        self.commands = []
        self.snapshot = "b" * 64
        self.corrupt = False
        self.restore_fail = False
        self.invalid_snapshot = False
        self.restored = []

    def write(self, path, value):
        path.write_text(value)
        path.chmod(0o600)

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

    def test_retrieved_full_restore_publishes_preserving_all_config_fields(self):
        self.run_backup()
        updated = json.loads(self.config.read_text())
        self.assertEqual({k: v for k, v in updated.items() if k != "backup_readiness"},
                         {k: v for k, v in self.original.items() if k != "backup_readiness"})
        proof = updated["backup_readiness"]
        self.assertEqual(proof["backup_id"], self.snapshot)
        self.assertEqual(proof["offhost_object"], "s3://private-bucket/sifto/database/snapshots/" + self.snapshot)
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
                     self.env_text.replace("private-bucket", "REPLACE_BUCKET")]:
            self.write(self.env, text)
            with self.assertRaises(self.m.BackupError):
                self.run_backup()
            self.assertEqual(self.commands, [])
            self.assert_old()

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
        self.write(self.config, json.dumps(self.original))
        self.write(self.env, self.env_text.replace("APPROVED=database", "APPROVED=application").replace("/sifto/database", "/sifto/application"))
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
        self.write(self.config, json.dumps(self.original))
        self.write(self.env, self.env_text.replace("APPROVED=database", "APPROVED=application").replace("/sifto/database", "/sifto/application"))
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
