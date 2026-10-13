#!/bin/sh
# Trusted pipeline checkout; the native executor remains a distinct authority.
set -eu
umask 077
test "${GITHUB_REF:-}" = refs/heads/main
test "${GITHUB_REPOSITORY:-}" = enjoydarts/sifto
test "${GITHUB_RUN_ATTEMPT:-}" = 1
case "${GITHUB_SHA:-}" in *[!0-9a-f]*|'') exit 1 ;; esac
test "${#GITHUB_SHA}" -eq 40
MAINTENANCE_CHECKED_OUT_SHA=$(git rev-parse HEAD)
export MAINTENANCE_CHECKED_OUT_SHA
test "$MAINTENANCE_CHECKED_OUT_SHA" = "$GITHUB_SHA"
test "$MAINTENANCE_CHECKED_OUT_SHA" = "${MAINTENANCE_PIPELINE_SHA:-}"
git diff --quiet HEAD -- infra/maintenance
test -z "$(git ls-files --others --exclude-standard -- infra/maintenance)"
# Reject credential widening even though only four secrets enter preparation.
test -z "${MAINTENANCE_APPLY_SSH_KEY:-}${MAINTENANCE_APPROVAL_PRIVATE_KEY:-}${MAINTENANCE_OPS_READ_KEY:-}${MAINTENANCE_ANTHROPIC_API_KEY:-}"
MAINTENANCE_CONTROLLER_UID=$(id -u)
MAINTENANCE_CONTROLLER_GID=$(id -g)
export MAINTENANCE_CONTROLLER_UID MAINTENANCE_CONTROLLER_GID
post_temp=$(mktemp -d)
MAINTENANCE_SECRET_DIR=$post_temp/secrets
MAINTENANCE_WORK_DIR=$post_temp/work
export MAINTENANCE_SECRET_DIR MAINTENANCE_WORK_DIR
compose() {
  docker compose -f infra/maintenance/compose.yml -f infra/maintenance/compose.post-verification.yml --profile controller "$@"
}
cleanup() {
  compose run --rm -T controller python -B -c 'from pathlib import Path; import shutil; [(shutil.rmtree(p) if p.is_dir() and not p.is_symlink() else p.unlink()) for p in Path("/private").iterdir()]' >/dev/null 2>&1 || true
  rm -rf "$post_temp" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM
mkdir -m 700 "$MAINTENANCE_SECRET_DIR" "$MAINTENANCE_WORK_DIR"
git ls-tree -r HEAD -- infra/maintenance/requirements.txt infra/maintenance/maintenance infra/maintenance/tests infra/maintenance/playbooks >"$MAINTENANCE_SECRET_DIR/source.txt"
chmod 600 "$MAINTENANCE_SECRET_DIR/source.txt"
if ! compose build controller post-preparation >"$post_temp/build.log" 2>&1; then
  echo maintenance_post_verification_build_failed_no_apply_retry >&2
  exit 1
fi
# Values are inherited by name, never put in argv or Docker build arguments.
status=0
compose run --rm -T \
  -e MAINTENANCE_SCAN_SSH_KEY -e MAINTENANCE_OPS_WRITE_KEY -e MAINTENANCE_HOSTS_JSON -e MAINTENANCE_KNOWN_HOSTS \
  -e INPUT_PRIVATE_COMMIT -e INPUT_PLAN_ID -e INPUT_PLAN_SHA256 -e INPUT_EXECUTOR_SHA \
  -e INPUT_APPROVAL_COMMIT -e INPUT_APPROVAL_SHA256 -e INPUT_DRY_RUN \
  post-preparation python -B -m maintenance.post_apply prepare >"$post_temp/prepare.log" 2>&1 || status=$?
if [ "$status" -eq 0 ]; then
  compose run --rm -T controller python -B -m maintenance.post_apply >"$post_temp/controller.log" 2>&1 || status=$?
fi
if [ -f "$MAINTENANCE_WORK_DIR/summary.md" ]; then
  compose run --rm -T controller python -B -c 'from pathlib import Path; print(Path("/private/summary.md").read_text(), end="")' >>"${GITHUB_STEP_SUMMARY:?}" 2>"$post_temp/export.log" || status=1
fi
if [ "$status" -ne 0 ]; then
  echo maintenance_post_verification_failed_no_apply_retry >&2
fi
exit "$status"
