"""Root host executor. Commands are fixed argv, never interpreted by a shell.

Root config at /etc/sifto-maintenance/config.json:
  host_id, trusted_public_key (base64 raw Ed25519), approved_workflow_sha (40 hex),
  state_path (/var/lib/sifto-maintenance), allowed_restart_services (unit names),
  host_role (application/database), database_enabled (maintenance enable bool),
  allowed_approvers (GitHub logins; empty denies apply),
  monitored_services (optional unit names).
Database readiness is root-maintained backup_readiness = {backup_id, host_id,
  verified_at (Unix seconds <=24h old), restore_tested_at (<=30 days old),
  offhost_object (s3:// or gs:// reference), offhost_sha256 (64 hex),
  offhost_verified_at (<=24h old)}.
The verification must include backup integrity and a successful isolated restore;
the agent never attempts a production restore. Enabling DB maintenance or renewing
readiness is an operator action; an approval cannot change these conditions.
"""

import base64
import contextlib
import fcntl
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time
import uuid

from .plan import (MaintenanceError, HASH, PACKAGE, SERVICE, SHA, VERSION, canonical_json,
                   expected_runtime, matches, normalize_closure, require, state_sha256, verify_envelope)

MAX_REQUEST_BYTES = 1024 * 1024
APT_LOCKS = ("/var/lib/dpkg/lock-frontend", "/var/lib/dpkg/lock",
             "/var/lib/apt/lists/lock", "/var/cache/apt/archives/lock")
APT_OPTIONS = ["-o", "DPkg::Lock::Timeout=0", "-o", "Acquire::Retries=0",
               "-o", "Acquire::http::Timeout=30", "-o", "Acquire::https::Timeout=30",
               "-o", "APT::Update::Error-Mode=any"]
CONTAINER_FORMAT = ('{"id":{{json .Id}},"image_id":{{json .Image}},'
                    '"image":{{json .Config.Image}},"status":{{json .State.Status}},'
                    '"health":{{if .State.Health}}{{json .State.Health.Status}}{{else}}"none"{{end}}}')


def strict_json(data):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "duplicate JSON key")
            result[key] = value
        return result

    def constant(_):
        raise MaintenanceError("nonfinite JSON value")

    try:
        return json.loads(data, object_pairs_hook=pairs, parse_constant=constant)
    except (ValueError, UnicodeError, RecursionError) as exc:
        raise MaintenanceError("invalid request JSON") from exc


def read_request(stream):
    data = stream.read(MAX_REQUEST_BYTES + 1)
    require(len(data) <= MAX_REQUEST_BYTES, "request exceeds size limit")
    request = strict_json(data or b"{}")
    require(type(request) is dict, "request must be an object")
    return request


def gateway_mode(gateway, original_command):
    allowed = {"scan-gateway": {"scan", "refresh", "report"},
               "apply-gateway": {"apply", "verify"}}
    require(gateway in allowed and original_command in allowed[gateway], "command denied by forced gateway")
    return original_command


def secure_path(path, *, directory=False):
    """Reject links and any ancestor writable by non-root (including parents)."""
    path = Path(path)
    require(path.is_absolute(), "path must be absolute")
    for candidate in [path, *path.parents]:
        info = candidate.lstat()
        require(not stat.S_ISLNK(info.st_mode) and info.st_uid == 0
                and info.st_mode & 0o022 == 0, "path must be root-owned and not writable by others")
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode), "invalid file type")


def load_config(path="/etc/sifto-maintenance/config.json"):
    require(os.geteuid() == 0, "host agent requires root")
    secure_path(path)
    require(Path(path).stat().st_mode & 0o077 == 0, "config must be root-only")
    config = strict_json(Path(path).read_bytes())
    required = {"host_id", "trusted_public_key", "approved_workflow_sha", "state_path",
                "allowed_restart_services", "database_enabled", "host_role"}
    require(type(config) is dict and required <= set(config), "missing root config")
    require(config["state_path"] == "/var/lib/sifto-maintenance", "invalid state path")
    require(matches(SHA, config["approved_workflow_sha"]), "invalid pinned executor SHA")
    require(type(config["database_enabled"]) is bool, "invalid database flag")
    require(config["host_role"] in ("application", "database"), "invalid host role")
    require(type(config.get("health_wait_seconds", 120)) is int
            and 0 <= config.get("health_wait_seconds", 120) <= 300, "invalid health wait")
    require(type(config.get("allowed_approvers", [])) is list
            and all(isinstance(x, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,99}", x)
                    for x in config.get("allowed_approvers", [])), "invalid configured approvers")
    for key in ("allowed_restart_services", "monitored_services"):
        if key in config:
            require(type(config[key]) is list and all(matches(SERVICE, x) for x in config[key]), "invalid configured services")
    try:
        require(len(base64.b64decode(config["trusted_public_key"], validate=True)) == 32, "invalid public key length")
    except (TypeError, ValueError) as exc:
        raise MaintenanceError("invalid public key") from exc
    return config


class SubprocessRunner:
    # Do not inherit PATH, PYTHONPATH, sudo caller environment or apt configuration.
    ENV = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL": "C", "LANG": "C",
           "DEBIAN_FRONTEND": "noninteractive", "NEEDRESTART_MODE": "l"}

    def __call__(self, argv, *, timeout=60, check=True):
        try:
            result = subprocess.run(argv, shell=False, env=self.ENV, stdin=subprocess.DEVNULL,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                    timeout=timeout, check=False, encoding="utf-8", errors="strict")
        except (OSError, subprocess.TimeoutExpired, UnicodeError) as exc:
            raise MaintenanceError("command execution failed or timed out") from exc
        require(not check or result.returncode == 0, "command failed: " + argv[0])
        require(len(result.stdout.encode("utf-8")) <= 16 * 1024 * 1024, "command output exceeds limit")
        return result.stdout

    def compare_versions(self, left, operator, right):
        result = subprocess.run(["dpkg", "--compare-versions", left, operator, right],
                                shell=False, env=self.ENV, stdin=subprocess.DEVNULL,
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        require(result.returncode in (0, 1), "version comparison failed")
        return result.returncode == 0


def probe_apt_locks():
    locked = []
    for path in APT_LOCKS:
        try:
            descriptor = os.open(path, os.O_RDWR | os.O_NOFOLLOW)
        except FileNotFoundError:
            continue  # No manager can hold a lock on a nonexistent pathname.
        try:
            try:
                fcntl.lockf(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                locked.append(path)
            else:
                fcntl.lockf(descriptor, fcntl.LOCK_UN)
        finally:
            os.close(descriptor)
    return locked


class HostAgent:
    def __init__(self, config, runner=None, *, clock=time.time, read_file=None, lock_probe=probe_apt_locks,
                 sleeper=time.sleep, monotonic=time.monotonic):
        self.config = config
        self.runner = runner or SubprocessRunner()
        self.clock = clock
        self.read_file = read_file or self._read_file
        self.lock_probe = lock_probe
        self.sleeper = sleeper
        self.monotonic = monotonic
        self.state = Path(config["state_path"])

    @staticmethod
    def _read_file(path):
        try:
            return Path(path).read_text()
        except FileNotFoundError:
            return ""

    def _mkdir(self, path):
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
        info = path.lstat()
        require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
                and info.st_mode & 0o077 == 0, "state directory must be private and owned by executor")

    @contextlib.contextmanager
    def locked(self, *, existing_only=False):
        if existing_only:
            for path in (self.state, self.state / "plans"):
                try:
                    info = path.lstat()
                except FileNotFoundError as exc:
                    raise MaintenanceError("dry-run requires provisioned maintenance state") from exc
                require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
                        and info.st_mode & 0o077 == 0, "invalid provisioned state directory")
        else:
            self._mkdir(self.state)
            self._mkdir(self.state / "plans")
        try:
            descriptor = os.open(self.state / "maintenance.lock", os.O_RDWR | os.O_NOFOLLOW
                                 | (0 if existing_only else os.O_CREAT), 0o600)
        except FileNotFoundError as exc:
            raise MaintenanceError("dry-run requires provisioned maintenance lock") from exc
        try:
            info = os.fstat(descriptor)
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                    and info.st_mode & 0o077 == 0, "invalid maintenance lock file")
            try:
                fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as exc:
                raise MaintenanceError("maintenance already running") from exc
            yield
        finally:
            os.close(descriptor)

    def _write(self, path, value, *, exclusive=False):
        flags = os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW
        if exclusive:
            flags |= os.O_EXCL
            target = path
        else:
            flags |= os.O_EXCL
            target = path.with_name(path.name + "." + str(uuid.uuid4()) + ".tmp")
        try:
            descriptor = os.open(target, flags, 0o600)
        except FileExistsError as exc:
            raise MaintenanceError("plan was already consumed") from exc
        try:
            with os.fdopen(descriptor, "wb") as stream:
                stream.write(canonical_json(value))
                stream.flush()
                os.fsync(stream.fileno())
            if not exclusive:
                os.replace(target, path)
            directory_fd = os.open(path.parent, os.O_DIRECTORY)
            try:
                os.fsync(directory_fd)
            finally:
                os.close(directory_fd)
        finally:
            if not exclusive and target.exists():
                target.unlink()

    def _metadata(self):
        path = self.state / "refresh.json"
        refreshed = strict_json(path.read_bytes())["refreshed_at"] if path.exists() else None
        return {"refreshed_at": refreshed,
                "fresh": type(refreshed) is int and 0 <= int(self.clock()) - refreshed < 86400}

    def inventory(self, *, simulation=True):
        platform = {}
        for line in self.read_file("/etc/os-release").splitlines():
            if "=" in line:
                key, value = line.split("=", 1)
                platform[key] = value.strip('"')
        actual_platform = {"id": platform.get("ID"), "version": platform.get("VERSION_ID"),
                           "architecture": self.runner(["dpkg", "--print-architecture"]).strip()}
        require(actual_platform == {"id": "ubuntu", "version": "24.04", "architecture": "arm64"}, "unsupported host platform")
        packages = []
        output = self.runner(["dpkg-query", "-W", "-f=${binary:Package}\t${Version}\t${db:Status-Status}\n"])
        names = set()
        for line in output.splitlines():
            parts = line.split("\t")
            require(len(parts) == 3, "malformed dpkg inventory")
            name, version, status_value = parts
            if status_value != "installed":
                continue
            require(matches(PACKAGE, name) and matches(VERSION, version) and name not in names, "invalid installed package")
            names.add(name)
            packages.append({"name": name, "version": version})
        container_ids = self.runner(["docker", "ps", "-aq", "--no-trunc"]).split()
        require(len(container_ids) <= 1000 and all(re.fullmatch(r"[0-9a-f]{64}", x) for x in container_ids), "invalid container IDs")
        containers = []
        if container_ids:
            output = self.runner(["docker", "inspect", "--format", CONTAINER_FORMAT, *container_ids])
            containers = [strict_json(line) for line in output.splitlines()]
            require(len(containers) == len(container_ids)
                    and {x["id"] for x in containers} == set(container_ids), "container inspection mismatch")
        services = {name: self.runner(["systemctl", "is-active", name], check=False).strip()
                    for name in self.config.get("monitored_services", ["docker.service", "ssh.service"])}
        engine = strict_json(self.runner(["docker", "version", "--format", "{{json .Server}} "]))
        runtime = {"boot_id": self.read_file("/proc/sys/kernel/random/boot_id").strip(),
                   "running_kernel": self.runner(["uname", "-r"]).strip(),
                   "installed_kernels": [x for x in packages if x["name"].startswith("linux-image-")],
                   "docker_version": engine["Version"], "containers": containers, "services": services}
        kernel_candidates = []
        for package in packages:
            match = re.fullmatch(r"linux-image-(?:unsigned-)?(\d.*)", package["name"])
            if match and match.group(1).rsplit("-", 1)[-1] == runtime["running_kernel"].rsplit("-", 1)[-1]:
                kernel_candidates.append((package["version"], match.group(1)))
        latest = None
        for version, kernel in kernel_candidates:
            if latest is None or self.runner.compare_versions(version, "gt", latest[0]):
                latest = (version, kernel)
        runtime["latest_installed_kernel"] = latest[1] if latest else runtime["running_kernel"]
        if self._database_host(packages):
            value = self._postgres_query("current_setting('server_version_num')")
            require(re.fullmatch(r"[0-9]{6}", value) is not None, "invalid PostgreSQL runtime version")
            runtime["postgres_version_num"] = value
        require(runtime["boot_id"] and runtime["running_kernel"] and runtime["docker_version"], "missing runtime identity")
        locks = self.lock_probe()
        require(not simulation or not locks, "apt/dpkg is busy")
        simulated = self.runner(["apt-get", "-s", "--no-remove", "--with-new-pkgs", "upgrade", *APT_OPTIONS], timeout=120) if simulation else ""
        return {"schema_version": 1, "host_id": self.config["host_id"], "platform": actual_platform,
                "packages": sorted(packages, key=lambda x: x["name"]), "runtime": runtime,
                "maintenance_policy": {"host_role": "database" if self._database_host(packages) else "application",
                                       "database_enabled": self.config["database_enabled"], "backup_ready": self._backup_ready()},
                "collected_at": int(self.clock()), "simulation": simulated, "metadata": self._metadata(),
                "apt_locks": locks, "reboot_required": bool(self.read_file("/var/run/reboot-required")),
                "reboot_required_packages": self.read_file("/var/run/reboot-required.pkgs").splitlines()}

    def _health(self, inventory, expected=None):
        require(all(value == "active" for value in inventory["runtime"]["services"].values()), "monitored service is inactive")
        current = {x["id"]: x for x in inventory["runtime"]["containers"]}
        expected = inventory if expected is None else expected
        for container in expected["runtime"]["containers"]:
            if container["status"] == "running":
                actual = current.get(container["id"])
                require(actual is not None and actual["status"] == "running"
                        and actual["image_id"] == container["image_id"] and actual["image"] == container["image"]
                        and actual["health"] in ("none", "healthy"), "container health check failed")
        if self._database_host(inventory["packages"]):
            require(self._postgres_query("1") == "1", "database read-only health check failed")

    def _postgres_query(self, expression):
        # Expressions come only from code constants, never request data/config.
        return self.runner(["sudo", "-n", "-u", "postgres", "/usr/bin/psql", "--no-psqlrc",
                            "--quiet", "--tuples-only", "--no-align", "--dbname=postgres", "--command",
                            "BEGIN READ ONLY; SELECT " + expression + "; COMMIT;"], timeout=30).strip()

    def _database_host(self, packages):
        return self.config["host_role"] == "database" or any(
            re.fullmatch(r"postgresql-\d+(?::[a-z0-9-]+)?", x["name"]) for x in packages)

    def _backup_ready(self):
        readiness = self.config.get("backup_readiness", {})
        now = int(self.clock())
        return bool(type(readiness) is dict
                    and readiness.get("host_id") == self.config["host_id"]
                    and isinstance(readiness.get("backup_id"), str)
                    and re.fullmatch(r"[A-Za-z0-9_.-]{1,128}", readiness["backup_id"])
                    and type(readiness.get("verified_at")) is int and 0 <= now - readiness["verified_at"] <= 86400
                    and type(readiness.get("restore_tested_at")) is int and 0 <= now - readiness["restore_tested_at"] <= 30 * 86400
                    and isinstance(readiness.get("offhost_object"), str)
                    and re.fullmatch(r"(?:s3|gs)://[A-Za-z0-9][A-Za-z0-9._/-]{2,1023}", readiness["offhost_object"])
                    and matches(HASH, readiness.get("offhost_sha256"))
                    and type(readiness.get("offhost_verified_at")) is int
                    and 0 <= now - readiness["offhost_verified_at"] <= 86400)

    @staticmethod
    def _runtime_targets(plan, inventory):
        actual = inventory["runtime"]
        require(all(actual.get(key) == value for key, value in plan["expected_runtime"].items()),
                "running kernel, Docker or PostgreSQL version does not match approved target")

    def _wait_healthy(self, before):
        deadline = self.monotonic() + self.config.get("health_wait_seconds", 120)
        while True:
            try:
                after = self.inventory(simulation=False)
                self._health(after, before)
                return after
            except MaintenanceError:
                if self.monotonic() >= deadline:
                    raise
                self.sleeper(min(2, max(0, deadline - self.monotonic())))

    def _policy(self, plan, inventory):
        require(set(plan["restart_services"]) <= set(self.config["allowed_restart_services"]), "restart service not whitelisted")
        majors = {match.group(1) for x in inventory["packages"]
                  if (match := re.match(r"postgresql-(\d+)(?:-|:|$)", x["name"]))}
        db_change = any(name.startswith("postgresql") for name in plan["restart_services"])
        for package in plan["packages"]:
            name = package["name"].split(":")[0]
            if name.startswith(("postgresql", "libpq", "pgbouncer", "pgpool")):
                db_change = True
                require(name not in ("postgresql", "postgresql-client", "postgresql-contrib", "postgresql-server-dev-all"),
                        "unversioned PostgreSQL metapackage is forbidden")
                match = re.match(r"postgresql-(?:(?:client|server-dev)-)?(\d+)(?:-|$)", name)
                require(match is None or match.group(1) in majors, "PostgreSQL major introduction is forbidden")
            if package["installed"] is not None:
                require(not self.runner.compare_versions(package["target"], "lt", package["installed"]), "package downgrade is forbidden")
        if db_change or self._database_host(inventory["packages"]):
            require(self.config["database_enabled"] and self._backup_ready(),
                    "database maintenance requires enable, verified offhost backup and isolated restore evidence")

    def _record_path(self, plan_id):
        try:
            require(str(uuid.UUID(plan_id)) == plan_id, "invalid plan ID")
        except (ValueError, AttributeError, TypeError) as exc:
            raise MaintenanceError("invalid plan ID") from exc
        return self.state / "plans" / (plan_id + ".json")

    def apply(self, request):
        require({"workflow_sha", "envelope"} <= set(request)
                and set(request) <= {"workflow_sha", "envelope", "dry_run"}, "invalid apply request fields")
        require(type(request.get("dry_run", False)) is bool, "dry_run must be boolean")
        require(request["workflow_sha"] == self.config["approved_workflow_sha"], "executor is not locally pinned")
        plan = verify_envelope(request["envelope"], self.config["trusted_public_key"], self.config["host_id"],
                               self.config["approved_workflow_sha"], now=int(self.clock()))
        require(request["envelope"]["approval"]["approved_by"] in self.config.get("allowed_approvers", []),
                "approver not permitted by root config")
        record_path = self._record_path(plan["plan_id"])
        require(not record_path.exists(), "plan was already consumed")
        before = self.inventory(simulation=False)
        require(state_sha256(before) == plan["state_sha256"], "host state drifted since plan")
        require(plan["expected_runtime"] == expected_runtime(before, plan["packages"], plan.get("action", "apt_upgrade")),
                "approved runtime targets do not match transaction")
        self._policy(plan, before)
        self._health(before)
        require(not self.lock_probe(), "apt/dpkg is busy")
        if plan.get("action", "apt_upgrade") == "apt_upgrade" and plan["packages"]:
            require(before["metadata"]["fresh"], "apt metadata must be refreshed before approval")
            specifications = [x["name"] + "=" + x["target"] for x in plan["packages"]]
            simulated = self.runner(["apt-get", "-s", "--no-remove", "install", *APT_OPTIONS, *specifications], timeout=120)
            require(normalize_closure(simulated, before["packages"]) == sorted(plan["packages"], key=lambda x: x["name"]), "approved dependency closure changed")
        else:
            specifications = []
        # Revalidate after the potentially slow solver. Refuse an expired approval or
        # a package/runtime change that happened while preparing the transaction.
        verify_envelope(request["envelope"], self.config["trusted_public_key"], self.config["host_id"],
                        self.config["approved_workflow_sha"], now=int(self.clock()))
        ready = self.inventory(simulation=False)
        require(state_sha256(ready) == plan["state_sha256"], "host state changed during preflight")
        require(not self.lock_probe(), "apt/dpkg became busy")
        self._policy(plan, ready)
        self._health(ready)
        if request.get("dry_run", False):
            return {"plan_id": plan["plan_id"], "status": "preflight_passed"}
        # Persist consumption BEFORE the first side effect. A crash leaves 'started',
        # permanently consumed. Failed/started records require operator diagnosis.
        record = {"plan": plan, "approval": request["envelope"]["approval"], "status": "started",
                  "started_at": int(self.clock()), "before_runtime": before["runtime"]}
        self._write(record_path, record, exclusive=True)
        try:
            if plan.get("action") == "reboot":
                record["status"] = "reboot_scheduled"
                self._write(record_path, record)
                self.runner(["systemctl", "reboot", "--no-block"], timeout=30)
            else:
                if specifications:
                    # Never kill apt/dpkg because a maintenance window elapsed.
                    # CI monitors externally; a disconnect leaves the consumed journal.
                    self.runner(["apt-get", "-y", "--no-remove", "install", *APT_OPTIONS, *specifications], timeout=None)
                for service in plan["restart_services"]:
                    self.runner(["systemctl", "restart", service], timeout=120)
                after = self._wait_healthy(before)
                actual = {x["name"]: x["version"] for x in after["packages"]}
                expected = {x["name"]: x["version"] for x in before["packages"]}
                expected.update({x["name"]: x["target"] for x in plan["packages"]})
                require(actual == expected, "installed package transaction verification failed")
                self._runtime_targets(plan, after)
                record["status"] = "completed"
                record["completed_at"] = int(self.clock())
                self._write(record_path, record)
        except Exception:
            record["status"] = "failed"
            record["failed_at"] = int(self.clock())
            self._write(record_path, record)
            raise
        return {"plan_id": plan["plan_id"], "status": record["status"]}

    def verify(self, request):
        require(set(request) == {"plan_id"}, "invalid verify request fields")
        path = self._record_path(request["plan_id"])
        require(path.is_file(), "unknown plan")
        record = strict_json(path.read_bytes())
        require(record["status"] in ("completed", "reboot_scheduled", "verified"), "failed or interrupted plan requires operator investigation")
        before = {"runtime": record["before_runtime"]}
        after = self._wait_healthy(before)
        if record["plan"].get("action") == "reboot":
            require(after["runtime"]["boot_id"] != before["runtime"]["boot_id"], "reboot has not completed")
        actual = {x["name"]: x["version"] for x in after["packages"]}
        require(all(actual.get(x["name"]) == x["target"] for x in record["plan"]["packages"]), "installed targets changed")
        self._runtime_targets(record["plan"], after)
        record["status"] = "verified"
        record["verified_at"] = int(self.clock())
        self._write(path, record)
        return {"plan_id": request["plan_id"], "status": "verified"}

    def run(self, mode, request):
        require(type(request) is dict, "request must be an object")
        if mode == "apply":
            require(type(request.get("dry_run", False)) is bool, "dry_run must be boolean")
        with self.locked(existing_only=mode == "apply" and request.get("dry_run") is True):
            if mode == "apply":
                return self.apply(request)
            if mode == "verify":
                return self.verify(request)
            require(mode in ("scan", "refresh", "report") and not request, "invalid scan request")
            if mode == "scan":
                return self.inventory()
            if mode == "refresh":
                require(not self.lock_probe(), "apt/dpkg is busy")
                self.runner(["apt-get", "update", "--error-on=any", *APT_OPTIONS], timeout=120)
                self._write(self.state / "refresh.json", {"refreshed_at": int(self.clock())})
                return {"status": "metadata_refreshed"}
            return {"host_id": self.config["host_id"], "metadata": self._metadata(),
                    "plans": [{"plan_id": path.stem, "status": strict_json(path.read_bytes())["status"]}
                              for path in sorted((self.state / "plans").glob("*.json"))]}
