#!/usr/bin/python3 -I
"""Installed at /opt/sifto-maintenance; stdout is private JSON, never a CI log."""

import os
from pathlib import Path
import sys

# -I prevents caller environment and writable cwd from affecting root imports.
sys.path.insert(0, str(Path(__file__).resolve().parent))

from maintenance.host import HostAgent, gateway_mode, load_config, read_request, secure_path
from maintenance.plan import MaintenanceError, canonical_json


def main():
    try:
        if len(sys.argv) != 2:
            raise MaintenanceError("exactly one forced gateway is required")
        mode = gateway_mode(sys.argv[1], os.environ.get("SSH_ORIGINAL_COMMAND", ""))
        secure_path("/opt/sifto-maintenance", directory=True)
        for source in ("host_agent.py", "maintenance/__init__.py", "maintenance/host.py", "maintenance/plan.py"):
            secure_path(Path("/opt/sifto-maintenance") / source)
        result = HostAgent(load_config()).run(mode, read_request(sys.stdin.buffer))
        sys.stdout.buffer.write(canonical_json(result) + b"\n")
        return 0
    except (MaintenanceError, OSError) as exc:
        # No argv, command output, inventory, private key or envelope in errors.
        sys.stderr.write("maintenance request rejected: " + str(exc) + "\n")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
