"""Exercise real controller playbooks with no credentials or external network."""
import os
from pathlib import Path
import subprocess


expected = (
    "Maintenance blocked: controller_operation_failed. Operator diagnosis required; "
    "no automatic apply retry.\n"
)
summary = Path("/private/summary.md")
assert os.environ.get("GITHUB_REF", "") == "", "fixture must reject before external operations"
assert not any(Path("/secrets").iterdir()), "fixture secrets must be empty"

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
    finally:
        # Delete root-owned 0600 output inside the container, including on failure,
        # so an unprivileged CI runner can remove its temporary fixture directory.
        summary.unlink(missing_ok=True)
    print(f"{operation}: real CLI reached configuration in read-only controller")
