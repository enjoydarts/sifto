#!/bin/sh
# GitHub runner wrapper. Secrets exist only in 0600 temporary files mounted RO.
set -eu
umask 077
operation=${1:-}
case "$operation" in scan|prepare|authorize|execute|audit) ;; *) echo maintenance_invalid_operation >&2; exit 1 ;; esac
test "${GITHUB_REF:-}" = refs/heads/main
case "${MAINTENANCE_EXECUTOR_SHA:-}" in *[!0-9a-f]*|'') exit 1 ;; esac
test "${#MAINTENANCE_EXECUTOR_SHA}" -eq 40
MAINTENANCE_CHECKED_OUT_SHA=$(git rev-parse HEAD)
export MAINTENANCE_CHECKED_OUT_SHA
test "$MAINTENANCE_CHECKED_OUT_SHA" = "$MAINTENANCE_EXECUTOR_SHA"
MAINTENANCE_CONTROLLER_UID=$(id -u)
MAINTENANCE_CONTROLLER_GID=$(id -g)
export MAINTENANCE_CONTROLLER_UID MAINTENANCE_CONTROLLER_GID
task_temp=$(mktemp -d)
MAINTENANCE_SECRET_DIR=$task_temp/secrets
MAINTENANCE_WORK_DIR=$task_temp/work
export MAINTENANCE_SECRET_DIR MAINTENANCE_WORK_DIR
cleanup() {
  # The controller owns private 0600 files. Remove them as its container UID;
  # remove host-created SSH/signing keys even if Docker is unavailable.
  docker compose -f infra/maintenance/compose.yml --profile controller run --rm -T controller python -B -c 'from pathlib import Path; import shutil; [(shutil.rmtree(p) if p.is_dir() and not p.is_symlink() else p.unlink()) for p in Path("/private").iterdir()]' >/dev/null 2>&1 || true
  rm -rf "$MAINTENANCE_SECRET_DIR" >/dev/null 2>&1 || true
  rm -rf "$task_temp" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM
mkdir -m 700 "$MAINTENANCE_SECRET_DIR" "$MAINTENANCE_WORK_DIR"
export MAINTENANCE_OPERATION=$operation
python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['MAINTENANCE_SECRET_DIR'])
operation = os.environ['MAINTENANCE_OPERATION']
secrets = {'known_hosts': 'MAINTENANCE_KNOWN_HOSTS'}
if operation in ('scan', 'execute'):
    secrets['hosts.json'] = 'MAINTENANCE_HOSTS_JSON'
    secrets['host_key'] = 'MAINTENANCE_SCAN_SSH_KEY' if operation == 'scan' else 'MAINTENANCE_APPLY_SSH_KEY'
secrets['ops_key'] = 'MAINTENANCE_OPS_WRITE_KEY' if operation in ('scan', 'authorize', 'audit') else 'MAINTENANCE_OPS_READ_KEY'
if operation == 'scan':
    secrets['anthropic_key'] = 'MAINTENANCE_ANTHROPIC_API_KEY'
if operation == 'authorize':
    secrets['approval_key'] = 'MAINTENANCE_APPROVAL_PRIVATE_KEY'
if operation == 'execute' and any(os.environ.get(key) for key in ('MAINTENANCE_APPROVAL_PRIVATE_KEY', 'MAINTENANCE_OPS_WRITE_KEY')):
    raise SystemExit('maintenance_executor_scope_invalid')
if operation == 'audit' and any(os.environ.get(key) for key in ('MAINTENANCE_SCAN_SSH_KEY', 'MAINTENANCE_APPLY_SSH_KEY', 'MAINTENANCE_HOSTS_JSON', 'MAINTENANCE_APPROVAL_PRIVATE_KEY', 'MAINTENANCE_OPS_READ_KEY', 'MAINTENANCE_ANTHROPIC_API_KEY', 'GH_TOKEN')):
    raise SystemExit('maintenance_audit_scope_invalid')
for filename, variable in secrets.items():
    with (root / filename).open('x') as stream:
        stream.write(os.environ.get(variable, '') + '\n')
    (root / filename).chmod(0o600)
inputs = {key: os.environ.get('INPUT_' + key.upper(), '') for key in ('private_commit', 'plan_id', 'plan_sha256', 'executor_sha', 'approval_commit', 'approval_sha256')}
for key in ('force_request', 'dry_run'):
    raw = os.environ.get('INPUT_' + key.upper(), 'false') or 'false'
    if raw not in ('true', 'false'):
        if operation != 'audit':
            raise SystemExit('maintenance_boolean_invalid')
        inputs[key] = None
    else:
        inputs[key] = raw == 'true'
if operation == 'audit':
    inputs['job_results'] = {job: os.environ.get('NEEDS_' + job.upper() + '_RESULT', '') for job in ('prepare', 'approve', 'execute')}
(root / 'inputs.json').write_text(json.dumps(inputs))
(root / 'inputs.json').chmod(0o600)
PY
# Source is pinned and build context excludes all private data.
if ! docker compose -f infra/maintenance/compose.yml --profile controller build controller >"$task_temp/build.log" 2>&1; then
  echo maintenance_controller_build_failed >&2
  exit 1
fi
case "$operation" in
  scan) set -- ansible-playbook -i localhost, playbooks/scan.yml ;;
  execute) set -- ansible-playbook -i localhost, playbooks/apply.yml ;;
  *) set -- python -B -m maintenance.cli "$operation" ;;
esac
status=0
docker compose -f infra/maintenance/compose.yml --profile controller run --rm -T controller "$@" >"$task_temp/controller.log" 2>&1 || status=$?
if [ -f "$MAINTENANCE_WORK_DIR/summary.md" ]; then
  if ! docker compose -f infra/maintenance/compose.yml --profile controller run --rm -T controller python -B -c 'from pathlib import Path; print(Path("/private/summary.md").read_text(), end="")' >>"${GITHUB_STEP_SUMMARY:?}" 2>"$task_temp/export.log"; then
    status=1
  fi
fi
if [ "$operation" = authorize ] && [ "$status" -eq 0 ]; then
  if ! docker compose -f infra/maintenance/compose.yml --profile controller run --rm -T controller python -B -c 'from pathlib import Path; import re; text=Path("/private/output").read_text(); assert re.fullmatch(r"approval_commit=[0-9a-f]{40}\napproval_sha256=[0-9a-f]{64}\n", text); print(text, end="")' >>"${GITHUB_OUTPUT:?}" 2>"$task_temp/export.log"; then
    status=1
  fi
fi
if [ "$status" -ne 0 ]; then
  echo maintenance_controller_failed_operator_diagnosis_required >&2
fi
exit "$status"
