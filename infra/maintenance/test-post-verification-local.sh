#!/bin/sh
# Run the production wrapper against a synthetic committed checkout/network none.
set -eu
umask 077
post_fixture=$(mktemp -d)
POST_REAL_DOCKER=$(command -v docker)
POST_FIXTURE=$post_fixture
export POST_REAL_DOCKER POST_FIXTURE
cleanup() {
  fixture_status=$?
  if [ "$fixture_status" -ne 0 ]; then
    for log in scope.log wrapper.log docker.log build.log; do
      if [ -f "$post_fixture/$log" ]; then cat "$post_fixture/$log" >&2; fi
    done
  fi
  "$POST_REAL_DOCKER" compose --env-file /dev/null --project-name sifto-post-verification-test \
    -f "$post_fixture/repository/infra/maintenance/compose.yml" \
    -f "$post_fixture/repository/infra/maintenance/compose.post-verification.yml" \
    -f "$post_fixture/network-none.yml" down --volumes >/dev/null 2>&1 || true
  rm -rf "$post_fixture"
}
trap cleanup EXIT HUP INT TERM
mkdir -m 700 "$post_fixture/repository" "$post_fixture/bin" "$post_fixture/tmp"
mkdir -p "$post_fixture/repository/infra"
cp -R infra/maintenance "$post_fixture/repository/infra/maintenance"
# Match actions/checkout's public source directory permissions. Private fixture
# state stays 0700; the capability-free caller must be able to import image code.
chmod -R a+rX "$post_fixture/repository/infra/maintenance"
# The fixture has no operator files or real credentials and commits only copied code.
git -C "$post_fixture/repository" init -q
git -C "$post_fixture/repository" -c user.name=Fixture -c user.email=fixture@example.invalid add infra
git -C "$post_fixture/repository" -c user.name=Fixture -c user.email=fixture@example.invalid commit -qm fixture
cat > "$post_fixture/network-none.yml" <<'EOF'
services:
  controller:
    network_mode: none
  post-preparation:
    network_mode: none
EOF
cat > "$post_fixture/bin/docker" <<'EOF'
#!/bin/sh
set -eu
test "$1" = compose
shift
# Preserve the first base file as the build context, but forward it only once.
# Some Compose versions reject repeated security_opt entries from duplicate -f.
test "${1:-}" = -f
test "${2:-}" = infra/maintenance/compose.yml
shift 2
previous=
for argument in "$@"; do
  if [ "$previous" = -f ] && [ "$argument" = infra/maintenance/compose.yml ]; then
    echo fixture_duplicate_base_rejected >&2
    exit 1
  fi
  previous=$argument
done
case "$*" in
  *'build controller post-preparation')
    # These are synthetic fixture-only logs. Preserve the actual Docker build
    # reason before the production wrapper correctly deletes its private logs.
    status=0
    "$POST_REAL_DOCKER" compose -f infra/maintenance/compose.yml -f "$POST_FIXTURE/network-none.yml" "$@" >"$POST_FIXTURE/build.log" 2>&1 || status=$?
    cat "$POST_FIXTURE/build.log"
    exit "$status"
    ;;
  *'post-preparation python -B -m maintenance.post_apply prepare')
    status=0
    "$POST_REAL_DOCKER" compose -f infra/maintenance/compose.yml -f "$POST_FIXTURE/network-none.yml" "$@" >"$POST_FIXTURE/docker.log" 2>&1 || status=$?
    cat "$POST_FIXTURE/docker.log"
    if [ "$status" -ne 0 ]; then
      "$POST_REAL_DOCKER" compose -f infra/maintenance/compose.yml -f infra/maintenance/compose.post-verification.yml \
        -f "$POST_FIXTURE/network-none.yml" --profile controller run --rm -T controller \
        python -B -c 'from pathlib import Path; print(Path("/private/error.txt").read_text())' >>"$POST_FIXTURE/docker.log" 2>&1 || true
    fi
    exit "$status"
    ;;
  *'controller python -B -m maintenance.post_apply')
    status=0
    "$POST_REAL_DOCKER" compose -f infra/maintenance/compose.yml -f "$POST_FIXTURE/network-none.yml" "$@" || status=$?
    "$POST_REAL_DOCKER" compose -f infra/maintenance/compose.yml \
      -f infra/maintenance/compose.post-verification.yml -f "$POST_FIXTURE/network-none.yml" \
      --profile controller run --rm -T -e PYTHONPATH=/src controller python -B tests/post_verification_scope_smoke.py >"$POST_FIXTURE/scope.log" 2>&1
    exit "$status"
    ;;
  *) exec "$POST_REAL_DOCKER" compose -f infra/maintenance/compose.yml -f "$POST_FIXTURE/network-none.yml" "$@" ;;
esac
EOF
chmod 700 "$post_fixture/bin/docker"
export PATH="$post_fixture/bin:$PATH"
export TMPDIR="$post_fixture/tmp"
export COMPOSE_PROJECT_NAME=sifto-post-verification-test
export GITHUB_REF=refs/heads/main GITHUB_REPOSITORY=enjoydarts/sifto GITHUB_RUN_ID=123 GITHUB_RUN_ATTEMPT=1
GITHUB_SHA=$(git -C "$post_fixture/repository" rev-parse HEAD)
MAINTENANCE_PIPELINE_SHA=$GITHUB_SHA
export GITHUB_SHA MAINTENANCE_PIPELINE_SHA
export MAINTENANCE_EXECUTOR_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
export MAINTENANCE_OPS_REPO=enjoydarts/fixture-evidence
export GH_TOKEN=fixture-token-never-public
export MAINTENANCE_SCAN_SSH_KEY=fixture-scan-key-never-public MAINTENANCE_OPS_WRITE_KEY=fixture-ops-key-never-public
export MAINTENANCE_HOSTS_JSON='[{"host_id":"fixture","role":"application","address":"192.0.2.1","port":22}]'
export MAINTENANCE_KNOWN_HOSTS=fixture-known-hosts-never-public
export MAINTENANCE_APPLY_SSH_KEY= MAINTENANCE_APPROVAL_PRIVATE_KEY= MAINTENANCE_OPS_READ_KEY= MAINTENANCE_ANTHROPIC_API_KEY=
export INPUT_PRIVATE_COMMIT=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb INPUT_PLAN_ID=11111111-1111-4111-8111-111111111111
export INPUT_PLAN_SHA256=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
export INPUT_EXECUTOR_SHA=$MAINTENANCE_EXECUTOR_SHA INPUT_APPROVAL_COMMIT=dddddddddddddddddddddddddddddddddddddddd
export INPUT_APPROVAL_SHA256=eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee INPUT_DRY_RUN=false
export GITHUB_STEP_SUMMARY="$post_fixture/summary.md"
# The interceptor must reject repeated base files before Compose can merge the
# security settings twice. Exercise its actual argv handling, not source text.
status=0
(cd "$post_fixture/repository" && docker compose -f infra/maintenance/compose.yml \
  -f infra/maintenance/compose.yml --profile controller config --quiet) >"$post_fixture/duplicate-base.log" 2>&1 || status=$?
test "$status" -ne 0
grep -q 'fixture_duplicate_base_rejected' "$post_fixture/duplicate-base.log"
status=0
(cd "$post_fixture/repository" && sh infra/maintenance/run-post-verification.sh) >"$post_fixture/wrapper.log" 2>&1 || status=$?
test "$status" -ne 0
test -f "$post_fixture/scope.log"
grep -q 'Pipeline image, distinct executor, credential scope' "$post_fixture/scope.log"
grep -q 'Do not retry apply' "$post_fixture/summary.md"
! grep -Eq 'fixture-(token|scan-key|ops-key|known-hosts)-never-public|Traceback' "$post_fixture/wrapper.log" "$post_fixture/summary.md" "$post_fixture/scope.log"
test -z "$(ls -A "$post_fixture/tmp")"
# Runtime credentials widening must stop before Docker runs.
export MAINTENANCE_APPLY_SSH_KEY=fixture-forbidden-key
status=0
(cd "$post_fixture/repository" && sh infra/maintenance/run-post-verification.sh) >"$post_fixture/forbidden.log" 2>&1 || status=$?
test "$status" -ne 0
echo 'Post-verification real wrapper: pipeline build, scoped credentials, network none and cleanup passed'
