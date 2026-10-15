import base64
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
import importlib.util
import marshal
import struct
import os
from unittest.mock import patch
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization
from maintenance import plan as p
from test_plan import SIMULATION
from test_container_identity import raw_identity

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
        self.containerd_version = "2.2.6"
        self.containerd_client_version = "v9.9.9"
        self.package_architectures = {}
        self.new_multiarch_packages = set()
        self.containers = None
        self.after_restart = None

    def __call__(self, argv, *, timeout=60, check=True, env=None, pass_fds=()):
        self.calls.append(list(argv))
        self.timeouts.append((list(argv), timeout))
        if argv[0] == "dpkg-query":
            packages = {"openssl": "3.0.13-0ubuntu3", "libssl3t64:arm64": "3.0.13-0ubuntu3"}
            statuses = {}
            for line in self.extra_packages.splitlines():
                name, version, status = line.split("\t")
                packages[name] = version
                statuses[name] = status
            uniform = "${Package}:${Architecture}" in argv[-1]
            if uniform:
                packages = {name if ":" in name else name + ":" + self.package_architectures.get(name, "arm64"): version
                            for name, version in packages.items()}
                statuses = {name if ":" in name else name + ":" + self.package_architectures.get(name, "arm64"): status
                            for name, status in statuses.items()}
            packages.update(self.installed_overrides)
            if not uniform:
                packages = {name + ":arm64" if name in self.new_multiarch_packages else name: version for name, version in packages.items()}
            else:
                packages = {name if ":" in name else name + ":" + self.package_architectures.get(name, "arm64"): version
                            for name, version in packages.items()}
            return "".join(f"{name}\t{version}\t{statuses.get(name, 'installed')}\n" for name, version in packages.items())
        if argv[:2] == ["dpkg", "--print-architecture"]:
            return "arm64\n"
        if argv[:2] == ["dpkg", "--compare-versions"]:
            return "false"  # Real runner exposes return code through check=False? use helper below.
        if argv[:2] == ["uname", "-r"]:
            return self.kernel + "\n"
        if argv[:2] == ["docker", "version"]:
            return json.dumps({"Version": self.docker_version}) + "\n"
        if argv[:2] == ["ctr", "--address"]:
            return f"Client:\n  Version: {self.containerd_client_version}\nServer:\n  Version: v{self.containerd_version}\n  Revision: fixture\n"
        if argv[:2] == ["docker", "ps"]:
            if self.containers is not None:
                return "".join(container["id"] + "\n" for container in self.containers)
            return "c" * 64 + "\n"
        if argv[:2] == ["docker", "inspect"]:
            if self.containers is not None:
                selected = []
                for container in self.containers:
                    value = copy.deepcopy(container)
                    value.setdefault('recreation_identity', raw_identity())
                    selected.append(value)
                return "".join(json.dumps(container) + "\n" for container in selected)
            return json.dumps({"id": "c" * 64, "image_id": self.image_id,
                               "image": "example/app:1", "status": "running", "health": self.container_health,
                               "recreation_identity": raw_identity()}) + "\n"
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
            if argv[0] == "/usr/bin/psql":
                return "1|fixture_db\n"
            if "server_version_num" in " ".join(argv):
                return self.pg_version_num + "\n"
            return "1\n"
        if argv[:2] == ["systemctl", "reboot"]:
            return ""
        if argv[:2] == ["systemctl", "restart"]:
            if self.after_restart:
                self.after_restart()
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
                       "allowed_restart_services": ["docker.service", "postgresql@16-main.service"],
                       "database_restart_services": ["postgresql@16-main.service"], "database_enabled": False}
        self.agent = h.HostAgent(self.config, self.runner, clock=lambda: 1000,
                                 read_file=self.read_file, lock_probe=lambda: [])

    def read_file(self, path):
        files = {"/etc/os-release": 'ID=ubuntu\nVERSION_ID="24.04"\n',
                 "/proc/sys/kernel/random/boot_id": "boot-a\n"}
        return files.get(str(path), "")

    def request(self, mutate=None, *, dry_run=False):
        self.agent.run("refresh", {})
        self.runner.calls.clear()
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], now=1000)
        if mutate:
            mutate(plan)
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123",
                    "approved_by": "human", "approved_at": 1000,
                    "expires_at": 2000, "workflow_sha": plan["workflow_sha"],
                    "execution_mode": "dry_run" if dry_run else "apply"}
        return {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key), "dry_run": dry_run}

    def mutations(self):
        return [cmd for cmd in self.runner.calls if (cmd[0] == "apt-get" and "-s" not in cmd)
                or cmd[:2] in (["systemctl", "restart"], ["systemctl", "reboot"])]

    def sentinel_containers(self):
        identity = raw_identity()
        identity['networks'][0]['aliases'] = ['fixture-alias']
        return [{"id": "a" * 64, "image_id": "sha256:" + "b" * 64,
                 "image": "docker.io/coollabsio/sentinel:1.0.2", "status": "running", "health": "healthy",
                 "recreation_identity": identity},
                {"id": "c" * 64, "image_id": "sha256:" + "d" * 64,
                 "image": "example/app:1", "status": "running", "health": "none"}]

    def sentinel_health_inventories(self):
        self.config["allowed_recreated_container_images"] = ["docker.io/coollabsio/sentinel:1.0.2"]
        self.runner.containers = self.sentinel_containers()
        before = self.agent.inventory(simulation=False)
        self.runner.containers[0]["id"] = "e" * 64
        return before, self.agent.inventory(simulation=False)

    def test_opted_in_sentinel_replacement_with_exact_image_is_healthy(self):
        before, after = self.sentinel_health_inventories()
        try:
            self.agent._health(after, before)
        except p.MaintenanceError as exc:
            self.fail(f"healthy opted-in Sentinel recreation was rejected: {exc}")

    def test_sentinel_replacement_requires_each_selected_metadata_field(self):
        before, after = self.sentinel_health_inventories()
        for path, value in [(('name',), '/other-fixture'), (('network_mode',), 'other-network'),
                            (('mounts', 0, 'source'), '/srv/other-fixture'),
                            (('mounts', 0, 'destination'), '/other-destination'),
                            (('mounts', 0, 'rw'), False), (('mounts', 0, 'propagation'), 'rshared'),
                            (('mounts', 1, 'volume_name'), 'other-volume'),
                            (('networks', 0, 'name'), 'other-network'),
                            (('networks', 0, 'network_id'), '2' * 64),
                            (('networks', 0, 'aliases'), ['other-alias']),
                            (('networks', 0, 'driver_options_present'), True),
                            (('networks', 0, 'static_ipam', 'ipv4_address'), '192.0.2.11')]:
            changed = copy.deepcopy(after)
            target = changed['runtime']['containers'][0]['recreation_identity']
            for key in path[:-1]:
                target = target[key]
            target[path[-1]] = value
            with self.subTest(path=path), self.assertRaisesRegex(p.MaintenanceError, 'container health check failed'):
                self.agent._health(changed, before)

    def test_legacy_receipt_cannot_authorize_recreation_but_existing_id_rules_remain(self):
        before, after = self.sentinel_health_inventories()
        del before['runtime']['containers'][0]['recreation_identity']
        preserved = copy.deepcopy(before)
        with self.assertRaisesRegex(p.MaintenanceError, 'container health check failed'):
            self.agent._health(after, before)
        self.assertEqual(before, preserved)
        after['runtime']['containers'][0]['id'] = before['runtime']['containers'][0]['id']
        after['runtime']['containers'][0]['recreation_identity']['name'] = '/other-fixture'
        self.agent._health(after, before)

    def test_sentinel_replacement_without_application_opt_in_is_rejected(self):
        before, after = self.sentinel_health_inventories()
        self.config.pop("allowed_recreated_container_images")
        with self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
            self.agent._health(after, before)
        for role, allowlist in [("application", []), ("application", ["docker.io/coollabsio/sentinel:1.0.3"]),
                                ("database", ["docker.io/coollabsio/sentinel:1.0.2"])]:
            with self.subTest(role=role):
                self.config.update(host_role=role, allowed_recreated_container_images=allowlist)
                with self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
                    self.agent._health(after, before)

    def test_ordinary_application_container_replacement_remains_rejected(self):
        before, after = self.sentinel_health_inventories()
        after["runtime"]["containers"][1]["id"] = "f" * 64
        with self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
            self.agent._health(after, before)

    def test_sentinel_replacement_with_changed_image_reference_or_id_is_rejected(self):
        before, after = self.sentinel_health_inventories()
        for field, value in [("image_id", "sha256:" + "f" * 64), ("image", "docker.io/coollabsio/sentinel:1.0.3")]:
            changed = copy.deepcopy(after)
            changed["runtime"]["containers"][0][field] = value
            with self.subTest(field=field), self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
                self.agent._health(changed, before)

    def test_ambiguous_original_or_replacement_sentinel_image_is_rejected(self):
        before, after = self.sentinel_health_inventories()
        for side in ("before", "after"):
            original, current = copy.deepcopy(before), copy.deepcopy(after)
            inventory = original if side == "before" else current
            duplicate = dict(inventory["runtime"]["containers"][0], id="f" * 64, status="exited")
            inventory["runtime"]["containers"].append(duplicate)
            with self.subTest(side=side), self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
                self.agent._health(current, original)

    def test_recreated_sentinel_requires_running_and_explicitly_healthy(self):
        before, after = self.sentinel_health_inventories()
        for status, health in [("running", "unhealthy"), ("running", "none"), ("running", "starting"), ("exited", "healthy")]:
            changed = copy.deepcopy(after)
            changed["runtime"]["containers"][0].update(status=status, health=health)
            with self.subTest(status=status, health=health), self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
                self.agent._health(changed, before)

    def test_existing_unhealthy_sentinel_id_cannot_fall_back_to_healthy_replacement(self):
        before, after = self.sentinel_health_inventories()
        after["runtime"]["containers"].append(dict(before["runtime"]["containers"][0], health="unhealthy"))
        with self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
            self.agent._health(after, before)

    def test_existing_sentinel_id_preserves_strict_identity_and_health_rules(self):
        before, after = self.sentinel_health_inventories()
        after["runtime"]["containers"][0]["id"] = before["runtime"]["containers"][0]["id"]
        after["runtime"]["containers"][0]["health"] = "none"
        self.agent._health(after, before)
        for field, value in [("image_id", "sha256:" + "f" * 64), ("image", "example/app:1"),
                             ("status", "exited"), ("health", "unhealthy")]:
            changed = copy.deepcopy(after)
            changed["runtime"]["containers"][0][field] = value
            with self.subTest(field=field), self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
                self.agent._health(changed, before)

    def sentinel_upgrade_request(self, *, dry_run=False):
        self.config["allowed_recreated_container_images"] = ["docker.io/coollabsio/sentinel:1.0.2"]
        self.runner.containers = self.sentinel_containers()
        self.runner.extra_packages = "docker.io\t28.0.0-1\tinstalled\n"
        self.runner.simulation = "Inst docker.io [28.0.0-1] (29.0.0-1 Docker:24.04/noble [arm64])\n"

        def restart():
            self.runner.docker_version = "29.0.0"
            self.runner.containers[0]["id"] = "e" * 64

        self.runner.after_restart = restart
        return self.request(dry_run=dry_run)

    def test_apply_and_verify_accept_sentinel_recreated_by_signed_docker_restart(self):
        request = self.sentinel_upgrade_request()
        self.assertEqual(request["envelope"]["plan"]["restart_services"], ["docker.service"])
        before_app = copy.deepcopy(self.runner.containers[1])
        try:
            result = self.agent.run("apply", request)
        except p.MaintenanceError as exc:
            self.fail(f"signed Docker restart with healthy Sentinel recreation failed: {exc}")
        self.assertEqual(result["status"], "completed")
        self.assertEqual(self.runner.containers[1], before_app)
        self.assertEqual(self.agent.run("verify", {"plan_id": result["plan_id"]})["status"], "verified")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)

    def test_metadata_drift_during_solver_denies_before_consumption(self):
        request = self.sentinel_upgrade_request()
        self.runner.before_simulation = lambda: self.runner.containers[0]['recreation_identity'].update(name='/other-fixture')
        with self.assertRaisesRegex(p.MaintenanceError, 'host state changed during preflight'):
            self.agent.run('apply', request)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(list((self.root / 'state/plans').glob('*.json')), [])

    def test_recreated_metadata_drift_after_restart_fails_and_cannot_be_repaired_into_verified(self):
        request = self.sentinel_upgrade_request()
        restart = self.runner.after_restart
        def changed_recreation():
            restart()
            self.runner.containers[0]['recreation_identity']['name'] = '/other-fixture'
        self.runner.after_restart = changed_recreation
        with self.assertRaisesRegex(p.MaintenanceError, 'container health check failed'):
            self.agent.run('apply', request)
        identifier = request['envelope']['plan']['plan_id']
        path = self.root / 'state/plans' / (identifier + '.json')
        failed = path.read_bytes()
        self.assertEqual(json.loads(failed)['status'], 'failed')
        self.runner.containers[0]['recreation_identity']['name'] = '/fixture-sentinel'
        with self.assertRaises(p.MaintenanceError):
            self.agent.run('verify', {'plan_id': identifier})
        with self.assertRaises(p.MaintenanceError):
            self.agent.run('apply', request)
        self.assertEqual(path.read_bytes(), failed)

    def test_opted_in_sentinel_upgrade_dry_run_preserves_containers_and_journal(self):
        request = self.sentinel_upgrade_request(dry_run=True)
        before = copy.deepcopy(self.runner.containers)
        self.assertEqual(self.agent.run("apply", request)["status"], "preflight_passed")
        self.assertEqual(self.runner.containers, before)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(list((self.root / "state" / "plans").glob("*.json")), [])

    def test_recreated_ordinary_app_after_restart_fails_and_cannot_be_verified_or_replayed(self):
        request = self.sentinel_upgrade_request()
        restart = self.runner.after_restart

        def replace_app():
            restart()
            self.runner.containers[1]["id"] = "f" * 64

        self.runner.after_restart = replace_app
        with self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
            self.agent.run("apply", request)
        plan_id = request["envelope"]["plan"]["plan_id"]
        path = self.root / "state" / "plans" / f"{plan_id}.json"
        failed_receipt = path.read_bytes()
        self.assertEqual(json.loads(failed_receipt)["status"], "failed")
        self.runner.containers[1]["id"] = "c" * 64
        with self.assertRaisesRegex(p.MaintenanceError, "failed or interrupted plan"):
            self.agent.run("verify", {"plan_id": plan_id})
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(path.read_bytes(), failed_receipt)

    def test_enabling_sentinel_opt_in_cannot_verify_or_replay_previous_failed_receipt(self):
        request = self.sentinel_upgrade_request()
        self.config.pop("allowed_recreated_container_images")
        with self.assertRaisesRegex(p.MaintenanceError, "container health check failed"):
            self.agent.run("apply", request)
        plan_id = request["envelope"]["plan"]["plan_id"]
        path = self.root / "state" / "plans" / f"{plan_id}.json"
        failed_receipt = path.read_bytes()
        self.assertEqual(json.loads(failed_receipt)["status"], "failed")
        self.config["allowed_recreated_container_images"] = ["docker.io/coollabsio/sentinel:1.0.2"]
        with self.assertRaisesRegex(p.MaintenanceError, "failed or interrupted plan"):
            self.agent.run("verify", {"plan_id": plan_id})
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(path.read_bytes(), failed_receipt)

    def load_root_config_with_recreated_images(self, **overrides):
        config = dict(self.config, state_path="/var/lib/sifto-maintenance",
                      trusted_public_key=base64.b64encode(self.key.public_key().public_bytes(
                          serialization.Encoding.Raw, serialization.PublicFormat.Raw)).decode())
        config.update(overrides)
        with tempfile.TemporaryDirectory(dir="/etc/sifto-maintenance") as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps(config))
            path.chmod(0o600)
            return h.load_config(path)

    def test_root_config_recreated_images_default_empty_and_valid_application_opt_in(self):
        self.assertEqual(self.load_root_config_with_recreated_images().get("allowed_recreated_container_images"), [])
        images = ["docker.io/coollabsio/sentinel:1.0.2", "docker.io/coollabsio/sentinel:12.34.56"]
        self.assertEqual(self.load_root_config_with_recreated_images(
            allowed_recreated_container_images=images)["allowed_recreated_container_images"], images)
        max_images = [f"docker.io/coollabsio/sentinel:1.0.{i}" for i in range(8)]
        self.assertEqual(self.load_root_config_with_recreated_images(
            allowed_recreated_container_images=max_images)["allowed_recreated_container_images"], max_images)
        self.assertEqual(self.load_root_config_with_recreated_images(
            host_role="database", allowed_recreated_container_images=[])["allowed_recreated_container_images"], [])

    def test_root_config_rejects_invalid_recreated_container_allowlists(self):
        image = "docker.io/coollabsio/sentinel:1.0.2"
        invalid = [None, image, {}, True, [None], [1], [image, image],
                   [f"docker.io/coollabsio/sentinel:1.0.{i}" for i in range(9)],
                   ["coollabsio/sentinel:1.0.2"], ["docker.io/other/sentinel:1.0.2"],
                   ["docker.io/coollabsio/other:1.0.2"], ["docker.io/coollabsio/sentinel:latest"],
                   ["docker.io/coollabsio/sentinel:1.0"], ["docker.io/coollabsio/sentinel:1.0.2-beta"],
                   [image + "\n"], [image + "@sha256:" + "a" * 64]]
        for value in invalid:
            with self.subTest(value=value), self.assertRaisesRegex(p.MaintenanceError, "recreated container"):
                self.load_root_config_with_recreated_images(allowed_recreated_container_images=value)
        with self.assertRaisesRegex(p.MaintenanceError, "recreated container"):
            self.load_root_config_with_recreated_images(host_role="database", allowed_recreated_container_images=[image])

    def test_bootstrap_opts_in_only_application_sentinel(self):
        bootstrap = Path(__file__).resolve().parents[1] / "bootstrap"
        app = json.loads((bootstrap / "config.example.json").read_text())
        database = json.loads((bootstrap / "config.database.example.json").read_text())
        self.assertEqual(app.get("allowed_recreated_container_images"), ["docker.io/coollabsio/sentinel:1.0.2"])
        self.assertEqual(database.get("allowed_recreated_container_images", []), [])

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
                                                         "backup_ready": True,
                                                         "allowed_restart_services": self.config["allowed_restart_services"],
                                                         "database_restart_services": self.config["database_restart_services"]})
        self.config["backup_readiness"]["offhost_verified_at"] = -90000
        self.assertFalse(self.agent.run("scan", {})["maintenance_policy"]["backup_ready"])

    def test_scan_returns_inventory_and_unknown_updates_when_apt_is_locked(self):
        self.agent.lock_probe = lambda: ["/var/lib/dpkg/lock-frontend"]
        try:
            inventory = self.agent.run("scan", {})
        except p.MaintenanceError as exc:
            self.fail(f"scan discarded readable inventory because apt was locked: {exc}")
        self.assertEqual(inventory["apt_locks"], ["/var/lib/dpkg/lock-frontend"])
        self.assertFalse(inventory["simulation_available"])
        self.assertEqual(inventory["simulation_error"], "apt_locked")
        self.assertEqual(inventory["simulation"], "")
        self.assertTrue(inventory["packages"])
        self.assertFalse(any(cmd[0] == "apt-get" for cmd in self.runner.calls))

    def test_scan_simulation_error_retains_inventory_and_blocks_planning(self):
        runner = self.runner

        def fail_simulation(argv, **kwargs):
            if argv[0] == "apt-get" and "-s" in argv:
                raise p.MaintenanceError("simulation unavailable")
            return runner(argv, **kwargs)

        self.agent.runner = fail_simulation
        try:
            inventory = self.agent.run("scan", {})
        except p.MaintenanceError as exc:
            self.fail(f"scan discarded readable inventory after simulation error: {exc}")
        self.assertFalse(inventory["simulation_available"])
        self.assertEqual(inventory["simulation_error"], "apt_simulation_failed")
        with self.assertRaises(p.MaintenanceError):
            p.build_plan(inventory, self.config["approved_workflow_sha"], now=1000)

    def test_apply_consumes_plan_before_exact_version_install_and_rejects_reuse(self):
        request = self.request()
        plan_id = request["envelope"]["plan"]["plan_id"]
        self.runner.before_install = lambda: self.assertTrue((self.root / "state" / "plans" / f"{plan_id}.json").is_file())
        result = self.agent.run("apply", request)
        self.assertEqual(result["status"], "completed")
        command = self.mutations()[0]
        self.assertIn("--no-remove", command)
        self.assertIn("openssl:arm64=3.0.13-0ubuntu3.5", command)
        self.assertIn("linux-modules-6.8.0-60-generic:arm64=6.8.0-60.63", command)
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(len(self.mutations()), 1)

    def test_new_native_multiarch_dependency_completes_and_verifies_exact_identity(self):
        self.runner.simulation = "Inst libnew (1.0 Ubuntu:24.04/noble [arm64])\nConf libnew (1.0 Ubuntu:24.04/noble [arm64])\n"
        self.runner.new_multiarch_packages.add("libnew")
        request = self.request()
        try:
            result = self.agent.run("apply", request)
        except p.MaintenanceError as exc:
            self.fail(f"approved native multiarch dependency was falsely failed after installation: {exc}")
        self.assertEqual(result["status"], "completed")
        self.assertEqual(request["envelope"]["plan"]["packages"][0]["name"], "libnew:arm64")
        self.assertEqual(self.agent.run("verify", {"plan_id": result["plan_id"]})["status"], "verified")

    def test_new_architecture_all_package_has_signed_all_identity(self):
        self.runner.simulation = "Inst datafiles (1.0 Ubuntu:24.04/noble [all])\nConf datafiles (1.0 Ubuntu:24.04/noble [all])\n"
        self.runner.package_architectures["datafiles"] = "all"
        request = self.request()
        self.assertEqual(request["envelope"]["plan"]["packages"][0]["name"], "datafiles:all")
        result = self.agent.run("apply", request)
        self.assertEqual(result["status"], "completed")
        self.assertIn("datafiles=1.0", self.mutations()[0])

    def test_pending_dpkg_state_blocks_preflight_without_mutation(self):
        request = self.request()
        for state in ["unpacked", "half-configured", "half-installed", "triggers-pending", "triggers-awaited"]:
            self.runner.extra_packages = f"pending\t1.0\t{state}\n"
            with self.subTest(state=state), self.assertRaises(p.MaintenanceError):
                self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(list((self.root / "state" / "plans").glob("*.json")), [])

    def test_pending_dpkg_scan_remains_readable_but_updates_are_unknown(self):
        self.runner.extra_packages = "pending\t1.0\tunpacked\n"
        inventory = self.agent.run("scan", {})
        self.assertIn("dpkg_state", inventory)
        self.assertFalse(inventory["dpkg_state"]["ready"])
        self.assertEqual(inventory["dpkg_state"]["pending"][0]["status"], "unpacked")
        self.assertFalse(inventory["simulation_available"])
        self.assertEqual(inventory["simulation_error"], "dpkg_pending")
        self.assertFalse(any(cmd[0] == "apt-get" for cmd in self.runner.calls))

    def test_reboot_pending_dpkg_state_is_denied(self):
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], allow_reboot=True, now=1000, action="reboot")
        approval = {"plan_sha256": p.plan_sha256(plan), "run_id": "123", "approved_by": "human", "approved_at": 1000,
                    "expires_at": 2000, "workflow_sha": plan["workflow_sha"], "execution_mode": "apply"}
        request = {"workflow_sha": plan["workflow_sha"], "envelope": p.sign_plan(plan, approval, self.key)}
        self.runner.extra_packages = "pending\t1.0\ttriggers-pending\n"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_running_containerd_server_version_is_collected_not_client_binary(self):
        inventory = self.agent.run("scan", {})
        self.assertIn("containerd_version", inventory["runtime"])
        self.assertEqual(inventory["runtime"]["containerd_version"], "2.2.6")

    def test_containerd_update_requires_new_running_server_version(self):
        self.config["allowed_restart_services"].append("containerd.service")
        self.runner.extra_packages = "containerd.io\t2.2.6-1~ubuntu.24.04\tinstalled\n"
        self.runner.simulation = "Inst containerd.io [2.2.6-1~ubuntu.24.04] (2.2.7-1~ubuntu.24.04 Docker:24.04/noble [arm64])\n"
        request = self.request()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertIn(["systemctl", "restart", "containerd.service"], self.mutations())
        record = json.loads(next((self.root / "state" / "plans").glob("*.json")).read_text())
        self.assertEqual(record["status"], "failed")

    def test_reboot_targets_installed_docker_version_even_when_current_daemon_is_older(self):
        self.runner.extra_packages = "docker.io\t29.1.3-0ubuntu1~24.04.1\tinstalled\n"
        inv = self.agent.run("scan", {})
        plan = p.build_plan(inv, self.config["approved_workflow_sha"], allow_reboot=True, now=1000, action="reboot")
        self.assertEqual(plan["expected_runtime"]["docker_version"], "29.1.3")

    def test_foreign_kernel_package_does_not_become_native_reboot_target(self):
        self.runner.extra_packages = "linux-image-6.8.0-99-generic:amd64\t6.8.0-99.99\tinstalled\n"
        inventory = self.agent.run("scan", {})
        self.assertEqual(inventory["runtime"]["latest_installed_kernel"], self.runner.kernel)

    def test_normal_removed_package_states_do_not_block_package_manager(self):
        self.runner.extra_packages = "removed\t1.0\tconfig-files\nmissing\t\tnot-installed\n"
        inventory = self.agent.run("scan", {})
        self.assertTrue(inventory["dpkg_state"]["ready"])
        self.assertTrue(inventory["simulation_available"])

    def test_containerd_upgrade_docker_restart_targets_already_installed_docker_version(self):
        self.config["allowed_restart_services"].append("containerd.service")
        self.runner.extra_packages = "docker.io\t29.1.3-0ubuntu1~24.04.1\tinstalled\ncontainerd.io\t2.2.6-1~ubuntu.24.04\tinstalled\n"
        self.runner.simulation = "Inst containerd.io [2.2.6-1~ubuntu.24.04] (2.2.7-1~ubuntu.24.04 Docker:24.04/noble [arm64])\n"
        request = self.request()
        self.assertEqual(request["envelope"]["plan"]["expected_runtime"]["docker_version"], "29.1.3")

    def test_invalid_signature_never_mutates(self):
        request = self.request()
        request["envelope"]["signature"] = "AAAA"
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_dry_run_verifies_transaction_without_consuming_or_mutating(self):
        request = self.request(dry_run=True)
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
        self.assertEqual(self.agent.run("apply", self.request())["status"], "completed")

    def test_dry_run_signed_mode_cannot_authorize_real_apply(self):
        request = self.request(dry_run=True)
        request["dry_run"] = False
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])
        self.assertFalse(self.agent._record_path(request["envelope"]["plan"]["plan_id"]).exists())
        request.pop("dry_run")
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

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
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"], "execution_mode": "dry_run"}
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

    def configure_pool_health(self):
        self.config["pgbouncer_health"] = {"database": "fixture_db", "user": "fixture_user"}
        credential = Path("/etc/sifto-maintenance/pgbouncer-health.pgpass")
        credential.write_text("127.0.0.1:6432:fixture_db:fixture_user:fixture-password\n")
        credential.chmod(0o600)
        self.addCleanup(credential.unlink, missing_ok=True)

    def test_database_role_rejects_library_upgrade_without_backup_even_when_enabled(self):
        self.config["host_role"] = "database"
        self.config["database_enabled"] = True
        request = self.request(dry_run=True)
        request["dry_run"] = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_role_rejects_all_upgrade_when_maintenance_disabled(self):
        self.config["host_role"] = "database"
        self.config["backup_readiness"] = self.readiness()
        request = self.request(dry_run=True)
        request["dry_run"] = True
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_role_requires_current_offhost_evidence(self):
        self.config["host_role"] = "database"
        self.config["database_enabled"] = True
        request = self.request(dry_run=True)
        request["dry_run"] = True
        for readiness in [{"backup_id": "backup-1", "verified_at": 900, "restore_tested_at": 800, "host_id": "sifto-app"},
                          {**self.readiness(), "offhost_verified_at": -90000},
                          {**self.readiness(), "offhost_sha256": "not-a-hash"}]:
            self.config["backup_readiness"] = readiness
            with self.subTest(readiness=readiness), self.assertRaises(p.MaintenanceError):
                self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])

    def test_database_role_valid_backup_allows_preflight_and_read_only_query(self):
        self.configure_pool_health()
        self.config["host_role"] = "database"
        self.config["database_enabled"] = True
        self.config["backup_readiness"] = self.readiness()
        request = self.request(dry_run=True)
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
        self.configure_pool_health()
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
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"], "execution_mode": "apply"}
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

    def install_entrypoint_fixture(self):
        installed = Path("/opt/sifto-maintenance")
        installed.mkdir(mode=0o755, exist_ok=True)
        self.addCleanup(shutil.rmtree, installed)
        source = Path(__file__).parents[1]
        shutil.copyfile(source / "host_agent.py", installed / "host_agent.py")
        shutil.copytree(source / "maintenance", installed / "maintenance", ignore=shutil.ignore_patterns("__pycache__"))
        state = Path("/var/lib/sifto-maintenance")
        (state / "plans").mkdir(mode=0o700, exist_ok=True)
        (state / "maintenance.lock").touch(mode=0o600)
        public_key = self.key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
        config = {**self.config, "state_path": str(state), "trusted_public_key": __import__("base64").b64encode(public_key).decode()}
        config_path = Path("/etc/sifto-maintenance/config.json")
        config_path.write_bytes(p.canonical_json(config))
        config_path.chmod(0o600)
        return installed

    def run_installed_entrypoint(self, installed):
        request = {"dry_run": True, "workflow_sha": self.config["approved_workflow_sha"], "envelope": {}}
        return subprocess.run([sys.executable, "-I", str(installed / "host_agent.py"), "apply-gateway"],
                              input=p.canonical_json(request), capture_output=True,
                              env={**os.environ, "SSH_ORIGINAL_COMMAND": "apply", "PYTHONDONTWRITEBYTECODE": "1"})

    def test_real_isolated_entrypoint_does_not_create_python_caches(self):
        installed = self.install_entrypoint_fixture()
        result = self.run_installed_entrypoint(installed)
        self.assertIn(b"invalid envelope", result.stderr)
        self.assertEqual(list(installed.rglob("*.pyc")), [], "first dry-run wrote Python bytecode")

    def test_real_isolated_entrypoint_rejects_untrusted_coolify_resources_before_local_import(self):
        installed = self.install_entrypoint_fixture()
        package = installed / "maintenance"
        marker = self.root / "unvalidated-package-imported"
        (package / "__init__.py").write_text(
            f"from pathlib import Path\nPath({str(marker)!r}).touch()\n")
        resources = ("coolify_rehearsal.py", "coolify_evidence.py", "coolify_docker.py",
                     "coolify-public-manifest.json", "coolify_probe.php")
        for name in resources:
            resource = package / name
            original = resource.read_bytes()
            for defect in ("missing", "group-writable", "other-writable", "nonroot", "symlink", "directory", "fifo"):
                with self.subTest(resource=name, defect=defect):
                    marker.unlink(missing_ok=True)
                    resource.unlink()
                    if defect == "directory":
                        resource.mkdir(mode=0o755)
                    elif defect == "fifo":
                        os.mkfifo(resource, mode=0o600)
                    elif defect == "symlink":
                        resource.symlink_to(package / "host.py")
                    elif defect != "missing":
                        resource.write_bytes(original)
                        resource.chmod({"group-writable": 0o664, "other-writable": 0o646}.get(defect, 0o644))
                        if defect == "nonroot":
                            os.chown(resource, 65534, 65534)
                    try:
                        result = self.run_installed_entrypoint(installed)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertFalse(marker.exists(), "local package imported before resource validation")
                        self.assertIn(b"maintenance bootstrap rejected", result.stderr)
                    finally:
                        if resource.is_dir() and not resource.is_symlink():
                            resource.rmdir()
                        else:
                            resource.unlink(missing_ok=True)
                        resource.write_bytes(original)
                        resource.chmod(0o644)
        try:
            package.chmod(0o777)
            marker.unlink(missing_ok=True)
            result = self.run_installed_entrypoint(installed)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(marker.exists(), "local package imported through writable ancestor")
            self.assertIn(b"maintenance bootstrap rejected", result.stderr)
        finally:
            package.chmod(0o755)

    def test_real_isolated_entrypoint_bypasses_timestamp_valid_malicious_cache(self):
        installed = self.install_entrypoint_fixture()
        host_source = installed / "maintenance" / "host.py"
        marker = self.root / "malicious-cache-executed"
        hostile_code = compile(f"from pathlib import Path\nPath({str(marker)!r}).touch()\n" + host_source.read_text(),
                               str(host_source), "exec")
        source_stat = host_source.stat()
        cache = Path(importlib.util.cache_from_source(str(host_source)))
        cache.parent.mkdir(mode=0o777)
        cache.parent.chmod(0o777)
        cache.write_bytes(importlib.util.MAGIC_NUMBER + struct.pack("<III", 0, int(source_stat.st_mtime), source_stat.st_size)
                          + marshal.dumps(hostile_code))
        cache.chmod(0o666)
        result = self.run_installed_entrypoint(installed)
        self.assertIn(b"invalid envelope", result.stderr)
        self.assertFalse(marker.exists(), "unchecked cached bytecode executed as root")

    def test_signed_docker_plan_missing_critical_restart_unit_is_denied_before_mutation(self):
        self.runner.extra_packages = "docker-ce\t5:28.0.0-1~ubuntu.24.04~noble\tinstalled\n"
        self.runner.simulation = "Inst docker-ce [5:28.0.0-1~ubuntu.24.04~noble] (5:28.1.1-1~ubuntu.24.04~noble Docker:24.04/noble [arm64])\n"
        request = self.request(lambda plan: plan.update(restart_services=[]))
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

    def test_expired_old_approval_never_mutates_or_consumes_at_entry(self):
        request = self.request()
        self.agent.clock = lambda: request['envelope']['plan']['expires_at'] + 86400
        self.runner.calls.clear()
        with self.assertRaises(p.MaintenanceError):
            self.agent.run("apply", request)
        self.assertEqual(self.mutations(), [])
        self.assertEqual(self.runner.calls, [])
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
                    "approved_at": 1000, "expires_at": 2000, "workflow_sha": plan["workflow_sha"], "execution_mode": "apply"}
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

    def test_execution_lock_blocks_scan_and_report_used_for_reproposal(self):
        other = h.HostAgent(self.config, self.runner, clock=lambda: 1000,
                            read_file=self.read_file, lock_probe=lambda: [])
        with self.agent.locked():
            for mode in ("scan", "report"):
                with self.subTest(mode=mode), self.assertRaises(p.MaintenanceError):
                    other.run(mode, {})
        self.assertEqual(self.runner.calls, [])
        self.assertEqual(list((self.root / "state" / "plans").glob("*.json")), [])

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
