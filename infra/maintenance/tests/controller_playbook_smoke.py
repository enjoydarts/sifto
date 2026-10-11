"""Exercise real controller playbooks with no credentials or external network."""
import os
from pathlib import Path
import pwd
import subprocess


expected = (
    "Maintenance blocked: controller_operation_failed. Operator diagnosis required; "
    "no automatic apply retry.\n"
)
summary = Path("/private/summary.md")
assert os.environ.get("GITHUB_REF", "") == "", "fixture must reject before external operations"
files = {path.name for path in Path("/secrets").iterdir()}
assert files in (set(), {"permission-probe"}), "fixture must contain no credentials"
if files:
    probe = Path("/secrets/permission-probe")
    assert probe.read_bytes() == b"fixture-only\n", "private fixture file must be readable"
    assert probe.stat().st_uid == 1001 and probe.stat().st_mode & 0o777 == 0o600
    for directory in (Path("/secrets"), Path("/private")):
        assert directory.stat().st_uid == 1001 and directory.stat().st_mode & 0o777 == 0o700
ssh = subprocess.run(
    ["ssh", "-G", "-F", "/dev/null", "controller-fixture.invalid"],
    capture_output=True, timeout=10,
)
assert ssh.returncode == 0, "controller UID must support OpenSSH configuration without networking"
assert pwd.getpwuid(os.getuid()).pw_uid == os.getuid()

for operation in ("scan", "apply"):
    summary.unlink(missing_ok=True)
    try:
        result = subprocess.run(
            ["ansible-playbook", "-i", "localhost,", f"playbooks/{operation}.yml"],
            capture_output=True, timeout=60,
        )
        # A task failure alone also occurs when Python cannot import maintenance.
        # This private summary is written only by the real CLI's exception handler;
        # valid playbook argv and an empty GITHUB_REF reach configuration's main guard.
        assert result.returncode == 2, f"{operation}: expected safe configuration rejection"
        assert summary.exists(), f"{operation}: CLI did not reach configuration"
        assert summary.read_text() == expected, f"{operation}: unexpected CLI summary"
        assert summary.stat().st_mode & 0o777 == 0o600, f"{operation}: summary must remain private"
    finally:
        # Delete private 0600 output inside the container, including on failure,
        # so an unprivileged CI runner can remove its temporary fixture directory.
        summary.unlink(missing_ok=True)
    print(f"{operation}: real CLI reached configuration in read-only controller")
