#!/bin/sh
# Exercise the production controller's read-only local Ansible connection.
set -eu
umask 077
controller_fixture=$(mktemp -d)
trap 'rm -rf "$controller_fixture"' EXIT HUP INT TERM
mkdir -m 0700 "$controller_fixture/secrets" "$controller_fixture/work"
cat > "$controller_fixture/network-none.yml" <<'EOF'
services:
  controller:
    network_mode: none
EOF
export MAINTENANCE_SECRET_DIR="$controller_fixture/secrets"
export MAINTENANCE_WORK_DIR="$controller_fixture/work"
# Empty fixtures and cleared controller inputs keep this check independent of
# operator credentials and production configuration in the caller environment.
env GITHUB_REF= GITHUB_RUN_ID= GITHUB_RUN_ATTEMPT= GH_TOKEN= \
    MAINTENANCE_EXECUTOR_SHA= MAINTENANCE_CHECKED_OUT_SHA= MAINTENANCE_PIPELINE_SHA= \
    MAINTENANCE_OPS_REPO= MAINTENANCE_ALLOWED_APPROVERS= MAINTENANCE_AI_MODEL= \
    docker compose --env-file /dev/null --project-name sifto-maintenance-controller-test \
    -f infra/maintenance/compose.yml -f "$controller_fixture/network-none.yml" \
    --profile controller run --build --rm -T --no-deps \
    controller python -B tests/controller_playbook_smoke.py
