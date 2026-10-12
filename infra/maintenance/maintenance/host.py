"""Root host executor. Commands are fixed argv, never interpreted by a shell.

Root config at /etc/sifto-maintenance/config.json:
  host_id, trusted_public_key (base64 raw Ed25519), approved_workflow_sha (40 hex),
  state_path (/var/lib/sifto-maintenance), allowed_restart_services (unit names),
  host_role (application/database), database_enabled (maintenance enable bool),
  allowed_approvers (GitHub logins; empty denies apply),
  allowed_recreated_container_images (optional; application-only Sentinel image
  references, empty by default),
  monitored_services (optional unit names), database_restart_services (explicit
  native PG cluster units, required for server/common/library updates),
  pgbouncer_health (optional {database, user}; identifiers only).
DB maintenance also requires the fixed root-owned 0600 credential file
/etc/sifto-maintenance/pgbouncer-health.pgpass and a successful read-only query
through 127.0.0.1:6432 into the configured application database. Credentials are
provisioned separately by root; they never travel through scan/apply requests.
Database readiness is root-maintained backup_readiness = {backup_id, host_id,
  verified_at (Unix seconds <=24h old), restore_tested_at (<=30 days old),
  offhost_object (s3:// or gs:// reference), offhost_sha256 (64 hex),
  offhost_verified_at (<=24h old)}.
The verification must include backup integrity and a successful isolated restore;
the agent never attempts a production restore. Enabling DB maintenance or renewing
readiness is an operator action; an approval cannot change these conditions.
"""

import base64
import codecs
import contextlib
import fcntl
import json
import os
from pathlib import Path
import re
import selectors
import stat
import subprocess
import time
import uuid

from .plan import (MaintenanceError, HASH, PACKAGE, SERVICE, SHA, VERSION, canonical_json, critical_restart_services,
                   expected_runtime, matches, normalize_closure, require, state_sha256, verify_envelope)

MAX_REQUEST_BYTES = 1024 * 1024
MAX_RECEIPT_HISTORY_COUNT = 10000
MAX_RECEIPT_HISTORY_BYTES = 32 * 1024 * 1024
APT_LOCKS = ("/var/lib/dpkg/lock-frontend", "/var/lib/dpkg/lock",
             "/var/lib/apt/lists/lock", "/var/cache/apt/archives/lock")
APT_OPTIONS = ["-o", "DPkg::Lock::Timeout=0", "-o", "Acquire::Retries=0",
               "-o", "Acquire::http::Timeout=30", "-o", "Acquire::https::Timeout=30",
               "-o", "APT::Update::Error-Mode=any"]
CONTAINER_FORMAT = ('{"id":{{json .Id}},"image_id":{{json .Image}},'
                    '"image":{{json .Config.Image}},"status":{{json .State.Status}},'
                    '"health":{{with index .State "Health"}}{{json .Status}}{{else}}"none"{{end}}}')
POOL_PASSFILE = "/etc/sifto-maintenance/pgbouncer-health.pgpass"
POOL_IDENTIFIER = re.compile(r"[A-Za-z_][A-Za-z0-9_-]{0,62}\Z")
RECREATED_CONTAINER_IMAGE = re.compile(r"docker\.io/coollabsio/sentinel:[0-9]+\.[0-9]+\.[0-9]+\Z")


def valid_pool_config(value):
    return (type(value) is dict and set(value) == {"database", "user"}
            and all(matches(POOL_IDENTIFIER, value[key]) for key in ("database", "user")))


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
    recreated_images = config.setdefault("allowed_recreated_container_images", [])
    require(type(recreated_images) is list and len(recreated_images) <= 8
            and all(matches(RECREATED_CONTAINER_IMAGE, image) for image in recreated_images)
            and len(set(recreated_images)) == len(recreated_images)
            and (not recreated_images or config["host_role"] == "application"),
            "invalid recreated container image allowlist")
    require(type(config.get("health_wait_seconds", 120)) is int
            and 0 <= config.get("health_wait_seconds", 120) <= 300, "invalid health wait")
    require(type(config.get("allowed_approvers", [])) is list
            and all(isinstance(x, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,99}", x)
                    for x in config.get("allowed_approvers", [])), "invalid configured approvers")
    for key in ("allowed_restart_services", "monitored_services", "database_restart_services"):
        if key in config:
            require(type(config[key]) is list and all(matches(SERVICE, x) for x in config[key]), "invalid configured services")
    require("pgbouncer_health" not in config or valid_pool_config(config["pgbouncer_health"]),
            "invalid pool health config")
    try:
        require(len(base64.b64decode(config["trusted_public_key"], validate=True)) == 32, "invalid public key length")
    except (TypeError, ValueError) as exc:
        raise MaintenanceError("invalid public key") from exc
    return config


class SubprocessRunner:
    # Do not inherit PATH, PYTHONPATH, sudo caller environment or apt configuration.
    ENV = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL": "C", "LANG": "C",
           "DEBIAN_FRONTEND": "noninteractive", "NEEDRESTART_MODE": "l"}
    MAX_STDOUT_BYTES = 16 * 1024 * 1024
    MAX_STDERR_BYTES = 16 * 1024 * 1024

    def __call__(self, argv, *, timeout=60, check=True, env=None, pass_fds=()):
        output = bytearray()
        exceeded = invalid_encoding = False
        try:
            with subprocess.Popen(argv, shell=False, env=self.ENV if env is None else env,
                                  pass_fds=pass_fds, stdin=subprocess.DEVNULL,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=0) as process:
                deadline = None if timeout is None else time.monotonic() + timeout
                try:
                    with selectors.DefaultSelector() as selector:
                        for stream, limit in ((process.stdout, self.MAX_STDOUT_BYTES),
                                              (process.stderr, self.MAX_STDERR_BYTES)):
                            os.set_blocking(stream.fileno(), False)
                            selector.register(stream, selectors.EVENT_READ,
                                              {"count": 0, "limit": limit,
                                               "decoder": codecs.getincrementaldecoder("utf-8")("strict")})
                        while selector.get_map():
                            remaining = None if deadline is None else deadline - time.monotonic()
                            if remaining is not None and remaining <= 0:
                                raise subprocess.TimeoutExpired(argv, timeout)
                            for key, _ in selector.select(remaining):
                                stream, state = key.fileobj, key.data
                                try:
                                    chunk = os.read(stream.fileno(), 64 * 1024)
                                except BlockingIOError:
                                    continue
                                state["count"] = min(state["count"] + len(chunk), state["limit"] + 1)
                                if state["count"] > state["limit"]:
                                    exceeded = True
                                    # Keep draining both pipes: APT/dpkg must never
                                    # receive SIGPIPE or be killed for noisy output.
                                    if stream is process.stdout:
                                        output.clear()
                                elif stream is process.stdout:
                                    output.extend(chunk)
                                if state["decoder"] is not None:
                                    try:
                                        state["decoder"].decode(chunk, final=not chunk)
                                    except UnicodeError:
                                        invalid_encoding = True
                                        state["decoder"] = None
                                if not chunk:
                                    selector.unregister(stream)
                                    stream.close()
                    remaining = None if deadline is None else max(0, deadline - time.monotonic())
                    process.wait(timeout=remaining)
                except (OSError, subprocess.TimeoutExpired):
                    # Preserve finite timeout behavior for probes. Install uses
                    # timeout=None and output validation never enters this path.
                    process.kill()
                    process.wait()
                    raise
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise MaintenanceError("command execution failed or timed out") from exc
        # Report validation only after completion; no stderr body or decoding
        # exception (which can contain credentials) is exposed to the caller.
        require(not invalid_encoding, "command execution failed or timed out")
        require(not check or process.returncode == 0, "command failed: " + argv[0])
        require(not exceeded, "command output exceeds limit")
        return output.decode("utf-8").replace("\r\n", "\n").replace("\r", "\n")

    def compare_versions(self, left, operator, right):
        result = subprocess.run(["dpkg", "--compare-versions", left, operator, right],
                                shell=False, env=self.ENV, stdin=subprocess.DEVNULL,
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        require(result.returncode in (0, 1), "version comparison failed")
        return result.returncode == 0


def probe_running_pgbouncer(runner):
    """Version of the executable actually backing the fixed systemd MainPID.

    Keep an open executable fd so upgrades that unlink the running old binary do
    not accidentally report the installed replacement. Fail if the process or
    executable identity changes while probing. No admin database auth is needed.
    """
    def main_pid():
        value = runner(["systemctl", "show", "--property=MainPID", "--value", "pgbouncer.service"], timeout=10).strip()
        require(re.fullmatch(r"[1-9][0-9]{0,6}", value) is not None and int(value) <= 4194304,
                "PgBouncer running PID is unavailable")
        return value

    pid = main_pid()
    executable = "/proc/" + pid + "/exe"
    process_stat = Path("/proc/" + pid + "/stat")

    def start_identity():
        fields = process_stat.read_text().rsplit(")", 1)[-1].split()
        require(len(fields) >= 20 and fields[19].isdigit(), "invalid PgBouncer process identity")
        return fields[19]

    try:
        started = start_identity()
        require(os.readlink(executable) in ("/usr/sbin/pgbouncer", "/usr/sbin/pgbouncer (deleted)"),
                "unexpected PgBouncer running executable")
        descriptor = os.open(executable, os.O_RDONLY | os.O_CLOEXEC)
        try:
            info = os.fstat(descriptor)
            require(stat.S_ISREG(info.st_mode) and info.st_uid == 0 and info.st_mode & 0o022 == 0,
                    "PgBouncer executable must be root-owned and protected")
            output = runner(["/proc/self/fd/" + str(descriptor), "--version"], timeout=10, pass_fds=(descriptor,))
            after = os.stat(executable)
            require(main_pid() == pid and start_identity() == started
                    and (after.st_dev, after.st_ino) == (info.st_dev, info.st_ino),
                    "PgBouncer running process changed during probe")
        finally:
            os.close(descriptor)
    except OSError as exc:
        raise MaintenanceError("PgBouncer running executable is unavailable") from exc
    first = output.splitlines()[0] if output.splitlines() else ""
    match = re.fullmatch(r"PgBouncer ([0-9]+\.[0-9]+\.[0-9]+)", first)
    require(match is not None, "invalid PgBouncer running version")
    return match.group(1)


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
        pending_packages = []
        output = self.runner(["dpkg-query", "-W", "-f=${Package}:${Architecture}\t${Version}\t${db:Status-Status}\n"])
        names = set()
        for line in output.splitlines():
            parts = line.split("\t")
            require(len(parts) == 3, "malformed dpkg inventory")
            name, version, status_value = parts
            if status_value not in ("installed", "config-files", "not-installed"):
                pending_packages.append({"name": name, "version": version, "status": status_value})
            if status_value != "installed":
                continue
            require(matches(PACKAGE, name) and ":" in name and matches(VERSION, version) and name not in names, "invalid installed package")
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
        ctr_output = self.runner(["ctr", "--address", "/run/containerd/containerd.sock", "version"], timeout=30)
        server = re.search(r"(?m)^Server:\s*\n(?:[ \t]+.*\n)*?[ \t]+Version:[ \t]+v?([0-9]+\.[0-9]+\.[0-9]+(?:[+.-][A-Za-z0-9_.+-]+)?)[ \t]*$", ctr_output)
        require(server is not None, "containerd running server version is unavailable")
        runtime = {"boot_id": self.read_file("/proc/sys/kernel/random/boot_id").strip(),
                   "running_kernel": self.runner(["uname", "-r"]).strip(),
                   "installed_kernels": [x for x in packages if x["name"].startswith("linux-image-")],
                   "docker_version": engine["Version"], "containerd_version": server.group(1),
                   "containers": containers, "services": services}
        kernel_candidates = []
        for package in packages:
            if package["name"].split(":")[1] != actual_platform["architecture"]:
                continue
            match = re.fullmatch(r"linux-image-(?:unsigned-)?(\d.*)", package["name"].split(":")[0])
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
        if any(x["name"].split(":")[0] == "pgbouncer" for x in packages):
            try:
                runtime["pgbouncer_version"] = probe_running_pgbouncer(self.runner)
            except MaintenanceError:
                runtime["pgbouncer_version"] = None
        if self._database_host(packages) or "pgbouncer_version" in runtime:
            runtime["database_pool_health"] = self._pool_health()
        require(runtime["boot_id"] and runtime["running_kernel"] and runtime["docker_version"], "missing runtime identity")
        locks = self.lock_probe()
        simulated = ""
        simulation_available = False
        simulation_error = "dpkg_pending" if simulation and pending_packages else "apt_locked" if simulation and locks else None
        if simulation and not locks and not pending_packages:
            try:
                simulated = self.runner(["apt-get", "-s", "--no-remove", "--with-new-pkgs", "upgrade", *APT_OPTIONS], timeout=120)
                simulation_available = True
            except MaintenanceError:
                simulation_error = "apt_simulation_failed"
        return {"schema_version": 1, "host_id": self.config["host_id"], "platform": actual_platform,
                "packages": sorted(packages, key=lambda x: x["name"]), "runtime": runtime,
                "dpkg_state": {"ready": not pending_packages, "pending": sorted(pending_packages, key=lambda x: x["name"])},
                "maintenance_policy": {"host_role": "database" if self._database_host(packages) else "application",
                                       "database_enabled": self.config["database_enabled"], "backup_ready": self._backup_ready(),
                                       "allowed_restart_services": self.config["allowed_restart_services"],
                                       "database_restart_services": self.config.get("database_restart_services", [])},
                "collected_at": int(self.clock()), "simulation": simulated, "simulation_available": simulation_available,
                "simulation_error": simulation_error, "metadata": self._metadata(),
                "apt_locks": locks, "reboot_required": bool(self.read_file("/var/run/reboot-required")),
                "reboot_required_packages": self.read_file("/var/run/reboot-required.pkgs").splitlines()}

    def _healthy_recreated_container(self, container, before, current):
        if (self.config["host_role"] != "application"
                or container["image"] not in self.config.get("allowed_recreated_container_images", [])):
            return None

        def same_image(candidate):
            return candidate["image"] == container["image"] and candidate["image_id"] == container["image_id"]

        originals = [candidate for candidate in before if same_image(candidate)]
        replacements = [candidate for candidate in current if same_image(candidate)]
        if len(originals) != 1 or len(replacements) != 1:
            return None
        replacement = replacements[0]
        if (replacement["id"] != container["id"] and replacement["status"] == "running"
                and replacement["health"] == "healthy"):
            return replacement
        return None

    def _health(self, inventory, expected=None):
        require(inventory["dpkg_state"]["ready"], "pending dpkg configuration requires operator recovery")
        require(all(value == "active" for value in inventory["runtime"]["services"].values()), "monitored service is inactive")
        current = {x["id"]: x for x in inventory["runtime"]["containers"]}
        expected = inventory if expected is None else expected
        for container in expected["runtime"]["containers"]:
            if container["status"] == "running":
                actual = current.get(container["id"])
                # A present original must pass the existing identity/health rules;
                # only a completely missing opted-in Sentinel may be recreated.
                if actual is None:
                    actual = self._healthy_recreated_container(container, expected["runtime"]["containers"],
                                                              inventory["runtime"]["containers"])
                require(actual is not None and actual["status"] == "running"
                        and actual["image_id"] == container["image_id"] and actual["image"] == container["image"]
                        and actual["health"] in ("none", "healthy"), "container health check failed")
        if self._database_host(inventory["packages"]):
            require(self._postgres_query("1") == "1", "database read-only health check failed")
        if self._database_host(inventory["packages"]) or "pgbouncer_version" in inventory["runtime"]:
            require(inventory["runtime"].get("database_pool_health", {}).get("status") == "healthy",
                    "application database pool read-only health check failed or unconfigured")
        if "pgbouncer_version" in inventory["runtime"]:
            require(isinstance(inventory["runtime"]["pgbouncer_version"], str), "PgBouncer running version is unavailable")

    def _pool_health(self):
        config = self.config.get("pgbouncer_health")
        if config is None:
            return {"status": "unknown", "reason": "unconfigured"}
        if not valid_pool_config(config):
            return {"status": "unknown", "reason": "invalid_configuration"}
        try:
            secure_path(POOL_PASSFILE)
            info = Path(POOL_PASSFILE).lstat()
            require(stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1, "invalid pool credentials")
        except (MaintenanceError, OSError):
            return {"status": "unknown", "reason": "credentials_unavailable"}
        # A clean environment prevents inherited passwords, services, options or
        # endpoint overrides. Identifiers cannot contain libpq conninfo/URIs.
        env = {**SubprocessRunner.ENV, "PGPASSFILE": POOL_PASSFILE, "PGCONNECT_TIMEOUT": "5"}
        try:
            result = self.runner(["/usr/bin/psql", "--no-psqlrc", "--no-password", "--quiet", "--tuples-only",
                                  "--no-align", "--set=ON_ERROR_STOP=1", "--host=127.0.0.1", "--port=6432",
                                  "--username=" + config["user"], "--dbname=" + config["database"], "--command",
                                  "BEGIN READ ONLY; SET LOCAL statement_timeout = '5s'; SELECT 1, current_database(); COMMIT;"],
                                 timeout=15, env=env).strip()
        except MaintenanceError:
            return {"status": "failed", "reason": "probe_failed"}
        if result != "1|" + config["database"]:
            return {"status": "failed", "reason": "route_mismatch"}
        return {"status": "healthy", "reason": "read_only_route_verified"}

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
                "running kernel or daemon version does not match approved target")

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
        require(inventory["dpkg_state"]["ready"], "pending dpkg configuration requires operator recovery")
        require(set(plan["restart_services"]) <= set(self.config["allowed_restart_services"]), "restart service not whitelisted")
        required = critical_restart_services(plan["packages"], inventory["maintenance_policy"])
        require(set(required) <= set(plan["restart_services"]), "critical service restart missing from signed scope")
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
            require(inventory["runtime"].get("database_pool_health", {}).get("status") == "healthy",
                    "database maintenance requires verified application pool health")

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
        require(request["envelope"]["approval"]["execution_mode"] ==
                ("dry_run" if request.get("dry_run", False) else "apply"), "signed execution mode mismatch")
        require(request["envelope"]["approval"]["approved_by"] in self.config.get("allowed_approvers", []),
                "approver not permitted by root config")
        record_path = self._record_path(plan["plan_id"])
        require(not record_path.exists(), "plan was already consumed")
        before = self.inventory(simulation=False)
        require(state_sha256(before) == plan["state_sha256"], "host state drifted since plan")
        require(plan["expected_runtime"] == expected_runtime(before, plan["packages"], plan.get("action", "apt_upgrade"), plan["restart_services"]),
                "approved runtime targets do not match transaction")
        self._policy(plan, before)
        self._health(before)
        require(not self.lock_probe(), "apt/dpkg is busy")
        if plan.get("action", "apt_upgrade") == "apt_upgrade" and plan["packages"]:
            require(before["metadata"]["fresh"], "apt metadata must be refreshed before approval")
            # :all is a signed architecture identity; apt selects these universal
            # packages by bare name. Native/foreign package selectors stay explicit.
            specifications = [(x["name"][:-4] if x["name"].endswith(":all") else x["name"]) + "=" + x["target"]
                              for x in plan["packages"]]
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

    def report(self):
        # Only the scan forced gateway exposes receipts, which the controller
        # stores privately. Bound records and bytes without touching journals.
        paths = list((self.state / "plans").glob("*.json"))
        response = {"host_id": self.config["host_id"], "metadata": self._metadata(), "plans": [],
                    "receipts": [], "receipt_total": len(paths), "receipt_truncated": len(paths),
                    "receipt_history_complete": False, "unresolved_execution_count": None}
        if len(paths) > MAX_RECEIPT_HISTORY_COUNT:
            return response
        for path in paths:
            info = path.lstat()
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                    and info.st_mode & 0o077 == 0, "invalid private receipt file")
            self._record_path(path.stem)
        paths.sort(key=lambda path: (path.stat().st_mtime_ns, path.name), reverse=True)
        size, history_size, unresolved = 0, 0, 0
        for path in paths:
            descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            with os.fdopen(descriptor, "rb") as stream:
                info = os.fstat(stream.fileno())
                require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                        and info.st_mode & 0o077 == 0 and info.st_size <= 8 * 1024 * 1024,
                        "invalid private receipt file")
                raw = stream.read(8 * 1024 * 1024 + 1)
            require(len(raw) <= 8 * 1024 * 1024, "private receipt exceeds limit")
            history_size += len(raw)
            if history_size > MAX_RECEIPT_HISTORY_BYTES:
                return response
            receipt = strict_json(raw)
            require(type(receipt) is dict and receipt.get("status") in
                    ("started", "failed", "completed", "reboot_scheduled", "verified"), "invalid receipt")
            diagnosis = receipt.get("operator_diagnosed_at")
            diagnosed = (type(diagnosis) is int and 0 < diagnosis <= int(self.clock())
                         and diagnosis >= receipt.get("failed_at", receipt.get("started_at", 0)))
            if receipt["status"] in ("started", "failed") and not diagnosed:
                unresolved += 1
            if len(response["receipts"]) < 20 and size + len(raw) <= 8 * 1024 * 1024:
                response["receipts"].append(receipt)
                response["plans"].append({"plan_id": path.stem, "status": receipt["status"]})
                size += len(raw)
                response["receipt_truncated"] -= 1
        response.update(receipt_history_complete=True, unresolved_execution_count=unresolved)
        return response

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
            return self.report()
