#!/usr/bin/python3 -I
"""Installed at /opt/sifto-maintenance; stdout is private JSON, never a CI log."""

import sys

if not sys.flags.isolated:
    sys.stderr.write("maintenance agent requires isolated Python (-I)\n")
    raise SystemExit(1)

# -I ignores PYTHONDONTWRITEBYTECODE and still reads timestamp-valid caches.
# A non-directory prefix prevents cache reads; disabling writes keeps dry-run pure.
sys.dont_write_bytecode = True
sys.pycache_prefix = "/dev/null"

import os
from pathlib import Path
import stat


def validate_installed_sources():
    # Only stdlib has been imported so far; never execute unvalidated local code.
    installed = Path("/opt/sifto-maintenance")
    if Path(__file__).absolute() != installed / "host_agent.py" or os.geteuid() != 0:
        raise ValueError("agent must run as root from trusted installation")
    for source in ("host_agent.py", "coolify_native_controller.py", "coolify_native_engine.py",
                   "maintenance/coolify_systemd.py", "maintenance/coolify_native_protocol.py",
                   "maintenance/coolify_cleanup.py", "maintenance/coolify_native_owner.py",
                   "systemd/sifto-coolify-native-controller.service",
                   "maintenance/__init__.py", "maintenance/host.py", "maintenance/plan.py",
                   "maintenance/coolify_host.py", "maintenance/coolify_plan.py",
                   "maintenance/coolify_rehearsal.py", "maintenance/coolify_evidence.py",
                   "maintenance/coolify_docker.py", "maintenance/coolify_rehearsal_engine.py",
                   "maintenance/coolify-public-manifest.json", "maintenance/coolify_probe.php",
                   "maintenance/coolify_startup.php", "maintenance/coolify_startup.py", "maintenance/coolify_startup_init",
                   "maintenance/coolify_startup_middleware.php", "maintenance/coolify_startup_nginx.conf",
                   "maintenance/coolify_horizon_inspect.php", "maintenance/coolify_startup_site.conf"):
        path = installed / source
        for candidate in [path, *path.parents]:
            info = candidate.lstat()
            if stat.S_ISLNK(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
                raise ValueError("agent sources must be root-owned with protected ancestors")
        if not stat.S_ISREG(path.lstat().st_mode) or path.lstat().st_nlink != 1:
            raise ValueError("invalid agent source type")


try:
    validate_installed_sources()
except (ValueError, OSError) as exc:
    sys.stderr.write("maintenance bootstrap rejected: " + str(exc) + "\n")
    raise SystemExit(1)

sys.path.insert(0, str(Path("/opt/sifto-maintenance")))

from maintenance.host import HostAgent, gateway_mode, load_config, read_request
from maintenance.plan import MaintenanceError, canonical_json


def main():
    try:
        if len(sys.argv) != 2:
            raise MaintenanceError("exactly one forced gateway is required")
        mode = gateway_mode(sys.argv[1], os.environ.get("SSH_ORIGINAL_COMMAND", ""))
        result = HostAgent(load_config()).run(mode, read_request(sys.stdin.buffer))
        sys.stdout.buffer.write(canonical_json(result) + b"\n")
        return 0
    except (MaintenanceError, OSError) as exc:
        # No argv, command output, inventory, private key or envelope in errors.
        sys.stderr.write("maintenance request rejected: " + str(exc) + "\n")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
