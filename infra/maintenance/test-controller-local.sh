#!/bin/sh
# Exercise the production controller's read-only local Ansible connection.
set -eu
umask 077
MAINTENANCE_CONTROLLER_UID=$(id -u)
MAINTENANCE_CONTROLLER_GID=$(id -g)
export MAINTENANCE_CONTROLLER_UID MAINTENANCE_CONTROLLER_GID
controller_fixture=$(mktemp -d)
cleanup() {
    docker compose --env-file /dev/null --project-name sifto-maintenance-controller-test \
        -f infra/maintenance/compose.yml -f "$controller_fixture/network-none.yml" \
        -f "$controller_fixture/linux-dac.yml" down --volumes >/dev/null 2>&1 || true
    rm -rf "$controller_fixture"
}
trap cleanup EXIT HUP INT TERM
mkdir -m 0700 "$controller_fixture/secrets" "$controller_fixture/work"
cat > "$controller_fixture/network-none.yml" <<'EOF'
services:
  controller:
    network_mode: none
EOF
cat > "$controller_fixture/linux-dac.yml" <<'EOF'
services:
  controller:
    volumes:
      - dac-secrets:/secrets:ro
      - dac-work:/private
  prepare-dac:
    image: sifto-maintenance-controller-test-controller
    user: "0:0"
    network_mode: none
    read_only: true
    volumes:
      - dac-secrets:/secrets
      - dac-work:/private
    command:
      - python
      - -B
      - -c
      - |
        import os
        from pathlib import Path
        for name in ('/secrets', '/private'):
            os.chown(name, 1001, 1001)
            os.chmod(name, 0o700)
        probe = Path('/secrets/permission-probe')
        probe.write_bytes(b'fixture-only\n')
        os.chown(probe, 1001, 1001)
        probe.chmod(0o600)
volumes:
  dac-secrets:
  dac-work:
EOF
export MAINTENANCE_SECRET_DIR="$controller_fixture/secrets"
export MAINTENANCE_WORK_DIR="$controller_fixture/work"
# Empty fixtures and cleared controller inputs keep this check independent of
# operator credentials and production configuration in the caller environment.
export GITHUB_REF= GITHUB_RUN_ID= GITHUB_RUN_ATTEMPT= GH_TOKEN= \
    MAINTENANCE_EXECUTOR_SHA= MAINTENANCE_CHECKED_OUT_SHA= MAINTENANCE_PIPELINE_SHA= \
    MAINTENANCE_OPS_REPO= MAINTENANCE_ALLOWED_APPROVERS= MAINTENANCE_AI_MODEL=
docker compose --env-file /dev/null --project-name sifto-maintenance-controller-test \
    -f infra/maintenance/compose.yml -f "$controller_fixture/network-none.yml" \
    --profile controller run --build --rm -T --no-deps \
    controller python -B tests/controller_playbook_smoke.py

# Named volumes exercise Linux DAC even on Docker Desktop's host bind mappings.
docker compose --env-file /dev/null --project-name sifto-maintenance-controller-test \
    -f infra/maintenance/compose.yml -f "$controller_fixture/network-none.yml" \
    -f "$controller_fixture/linux-dac.yml" run --rm -T --no-deps prepare-dac
env MAINTENANCE_CONTROLLER_UID=1001 MAINTENANCE_CONTROLLER_GID=1001 \
    docker compose --env-file /dev/null --project-name sifto-maintenance-controller-test \
    -f infra/maintenance/compose.yml -f "$controller_fixture/network-none.yml" \
    -f "$controller_fixture/linux-dac.yml" --profile controller run --build --rm -T --no-deps \
    controller python -B tests/controller_playbook_smoke.py

# Matching ownership grants access; container root without capabilities must
# still be denied access to the same 0700 directories and 0600 synthetic file.
docker compose --env-file /dev/null --project-name sifto-maintenance-controller-test \
    -f infra/maintenance/compose.yml -f "$controller_fixture/network-none.yml" \
    -f "$controller_fixture/linux-dac.yml" --profile controller run --rm -T --no-deps \
    --user 0:0 controller python -B -c '
from pathlib import Path
for operation in (lambda: Path("/secrets/permission-probe").read_bytes(),
                  lambda: Path("/private/denial-probe").write_bytes(b"fixture-only")):
    try:
        operation()
    except PermissionError:
        pass
    else:
        raise AssertionError("capability-free root must not bypass private fixture DAC")
print("Linux DAC: capability-free root denied private read and write")
'
